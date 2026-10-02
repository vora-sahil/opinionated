package structure_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vora-sahil/opinionated/internal/check"
	"github.com/vora-sahil/opinionated/internal/fsx"
	"github.com/vora-sahil/opinionated/internal/structure"
	"github.com/vora-sahil/opinionated/internal/testrepo"
)

func run(t *testing.T, spec string, files map[string]string) string {
	t.Helper()
	root := testrepo.New(t, files)
	s, probs, err := structure.Parse([]byte(spec))
	if err != nil || len(probs) > 0 {
		t.Fatalf("parse: %v %v", err, probs)
	}
	list, err := fsx.List(root)
	if err != nil {
		t.Fatal(err)
	}
	lay, v := s.Check(list)
	v = append(v, lay.CheckImports(check.NewSource(root), list, nil)...)
	check.Sort(v)
	var b strings.Builder
	for _, x := range v {
		fmt.Fprintf(&b, "%s:%d %s | %s\n", x.File, x.Line, x.ID, x.Message)
	}
	return b.String()
}

func expect(t *testing.T, got string, wants ...string) {
	t.Helper()
	lines := strings.Count(got, "\n")
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
	if lines != len(wants) {
		t.Errorf("got %d findings, want %d:\n%s", lines, len(wants), got)
	}
}

func TestShapeAndNaming(t *testing.T) {
	spec := `
tree:
  src/:
    closed: true
    files: kebab-case
    dirs: kebab-case
    api/: { role: interface, doc: HTTP handlers }
    features/:
      "{feature}/":
        requires: [index.ts]
  docs/:
`
	got := run(t, spec, map[string]string{
		"src/api/user-routes.ts":         "",
		"src/api/UserRoutes.ts":          "",
		"src/helpers/x.ts":               "",
		"src/features/billing/index.ts":  "",
		"src/features/Bad_Name/thing.ts": "",
		"scripts/Anything.sh":            "",
	})
	expect(t, got,
		`src/api/UserRoutes.ts:0 naming/structure | file name "UserRoutes.ts" is not kebab-case`,
		`src/helpers:0 layout/structure | src/helpers/ is not part of the declared structure; src/ allows: api/, features/`,
		`src/features/Bad_Name:0 naming/structure | directory name "Bad_Name" is not kebab-case`,
		`src/features/Bad_Name:0 layout/structure | src/features/Bad_Name/ must contain index.ts`,
	)
}

const layered = `
tree:
  src/:
    api/:    { role: interface }
    jobs/:   { role: worker }
    domain/: { role: core }
    infra/:  { role: adapter }
    features/:
      "{feature}/": { role: feature, isolated: true }
layers:
  - interface | worker
  - feature
  - adapter + core
aliases:
  "@/": src/
`

func TestLayersJS(t *testing.T) {
	got := run(t, layered, map[string]string{
		"src/api/routes.ts":           "import { pay } from '../features/billing/pay'\nimport x from 'react'\nimport { run } from '@/jobs/run'\n",
		"src/jobs/run.ts":             "export const run = 1\n",
		"src/domain/order.ts":         "import { db } from '../infra/db'\nimport { r } from '../api/routes'\n",
		"src/infra/db.ts":             "const o = require('../domain/order')\n",
		"src/features/billing/pay.ts": "import { cart } from '../cart/cart'\nimport { o } from '@/domain/order'\n",
		"src/features/cart/cart.ts":   "import './cart.css'\n",
	})
	expect(t, got,
		"src/api/routes.ts:3 boundaries/layers | interface must not import worker: they are independent",
		"src/domain/order.ts:2 boundaries/layers | core must not import interface: imports only point down the layers",
		"src/features/billing/pay.ts:1 boundaries/layers | src/features/billing/ must not import its sibling src/features/cart/",
	)
}

func TestLayersQualifiedNames(t *testing.T) {
	spec := `
tree:
  app/:
    src/:
      main/:
        kotlin/:
          com/:
            acme/:
              domain/: { role: core }
              web/:    { role: interface }
  svc/:
    domain/: { role: core }
    web/:    { role: interface }
  internal/:
    domain/: { role: core }
    web/:    { role: interface }
layers:
  - interface
  - core
`
	got := run(t, spec, map[string]string{
		"go.mod": "module example.com/shop\n\ngo 1.22\n",
		"app/src/main/kotlin/com/acme/domain/Order.kt":        "package com.acme.domain\nimport com.acme.web.OrderController\nimport org.springframework.web.Thing\n",
		"app/src/main/kotlin/com/acme/web/OrderController.kt": "import com.acme.domain.Order\n",
		"svc/domain/order.py":                                 "from svc.web import views\nfrom . import models\nimport web3\n",
		"svc/web/views.py":                                    "from ..domain import order\n",
		"internal/domain/order.go":                            "package domain\n\nimport (\n\t\"fmt\"\n\tw \"example.com/shop/internal/web\"\n\t\"github.com/other/web\"\n)\n",
		"internal/web/h.go":                                   "package web\n\nimport \"example.com/shop/internal/domain\"\n",
	})
	expect(t, got,
		"app/src/main/kotlin/com/acme/domain/Order.kt:2 boundaries/layers | core must not import interface",
		"svc/domain/order.py:1 boundaries/layers | core must not import interface",
		"internal/domain/order.go:5 boundaries/layers | core must not import interface",
	)
}

func TestParseProblems(t *testing.T) {
	_, probs, err := structure.Parse([]byte("tree:\n  src/:\n    colour: red\n    files: Title\n    api/: {role: a, isolated: true}\nlayers:\n  - a | ghost\nextra: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(probs, "\n")
	for _, w := range []string{`unknown key "colour"`, `files "Title"`, "isolated only applies", `role "ghost" is not assigned`, `unknown key "extra"`} {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}

func TestRenderAndGuide(t *testing.T) {
	s, _, _ := structure.Parse([]byte(layered))
	if got := s.LayerSummary(); got != "interface | worker → feature → adapter + core" {
		t.Errorf("summary = %q", got)
	}
	if !strings.Contains(s.Render(), "<feature>/") {
		t.Errorf("render:\n%s", s.Render())
	}
}

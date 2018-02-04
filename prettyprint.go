package corelang

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	. "github.com/metaleap/go-corelang/syn"
)

type InterpPrettyPrint struct {
	curIndent int
}

func (me *InterpPrettyPrint) Mod(mod *SynMod, args ...interface{}) (interface{}, error) {
	var buf bytes.Buffer
	for _, def := range mod.Defs {
		me.curIndent = 0
		me.def(&buf, def)
		buf.WriteString("\n\n")
	}
	return buf.String(), nil
}

func (me *InterpPrettyPrint) def(w *bytes.Buffer, def *SynDef, _ ...interface{}) {
	w.WriteString(def.Name)
	for _, defarg := range def.Args {
		w.WriteRune(' ')
		w.WriteString(defarg)
	}
	w.WriteString(" =\n")
	me.curIndent++
	w.WriteString(strings.Repeat("  ", me.curIndent))
	me.expr(w, def.Body, false)
	me.curIndent--
}

func (me *InterpPrettyPrint) Def(def *SynDef, args ...interface{}) (interface{}, error) {
	var buf bytes.Buffer
	me.def(&buf, def, args...)
	return buf.String(), nil
}

func (me *InterpPrettyPrint) expr(w *bytes.Buffer, expression IExpr, parensUnlessAtomic bool) {
	if parensUnlessAtomic && !expression.IsAtomic() {
		w.WriteRune('(')
	}
	switch expr := expression.(type) {
	case *ExprIdent:
		if expr.OpLike && expr.OpLone {
			w.WriteRune('(')
		}
		w.WriteString(expr.Val)
		if expr.OpLike && expr.OpLone {
			w.WriteRune(')')
		}
	case *ExprLitFloat:
		w.WriteString(strconv.FormatFloat(expr.Val, 'g', -1, 64))
	case *ExprLitUInt:
		if expr.Base == 16 {
			w.WriteString("0x")
		} else if expr.Base == 8 {
			w.WriteRune('0')
		}
		w.WriteString(strconv.FormatUint(expr.Val, expr.Base))
	case *ExprLitText:
		w.WriteString(strconv.Quote(expr.Val))
	case *ExprLitRune:
		w.WriteString(strconv.QuoteRune(expr.Val))
	case *ExprLambda:
		w.WriteString("\\")
		for _, lamarg := range expr.Args {
			w.WriteString(lamarg)
			w.WriteRune(' ')
		}
		w.WriteString("-> ")
		me.expr(w, expr.Body, true)
	case *ExprCall:
		me.expr(w, expr.Callee, true)
		w.WriteRune(' ')
		me.expr(w, expr.Arg, true)
	case *ExprLetIn:
		w.WriteString("LET\n")
		me.curIndent++
		for _, letdef := range expr.Defs {
			w.WriteString(strings.Repeat("  ", me.curIndent))
			me.def(w, letdef)
			w.WriteRune('\n')
		}
		me.curIndent--
		w.WriteString(strings.Repeat("  ", me.curIndent))
		w.WriteString("IN\n")
		me.curIndent++
		w.WriteString(strings.Repeat("  ", me.curIndent))
		me.expr(w, expr.Body, false)
		me.curIndent--
	case *ExprCtor:
		w.WriteRune('(')
		w.WriteString(strconv.FormatUint(expr.Tag, 10))
		w.WriteRune(' ')
		w.WriteString(strconv.FormatUint(expr.Arity, 10))
		w.WriteRune(')')
	case *ExprCaseOf:
		w.WriteString("CASE ")
		me.expr(w, expr.Scrut, false)
		w.WriteString(" OF\n")
		me.curIndent++
		w.WriteString(strings.Repeat("  ", me.curIndent))
		for _, alt := range expr.Alts {
			w.WriteString(strconv.FormatUint(alt.Tag, 10))
			for _, bind := range alt.Binds {
				w.WriteRune(' ')
				w.WriteString(bind)
			}
			w.WriteString(" ->\n")
			me.curIndent++
			w.WriteString(strings.Repeat("  ", me.curIndent))
			me.expr(w, alt.Body, false)
			me.curIndent--
			w.WriteRune('\n')
			w.WriteString(strings.Repeat("  ", me.curIndent))
		}
		me.curIndent--
	default:
		panic(fmt.Errorf("unknown expression type %T — %#v", expr, expr))
	}
	if parensUnlessAtomic && !expression.IsAtomic() {
		w.WriteRune(')')
	}
	return
}

func (me *InterpPrettyPrint) Expr(expr IExpr) (interface{}, error) {
	var buf bytes.Buffer
	me.expr(&buf, expr, false)
	return buf.String(), nil
}

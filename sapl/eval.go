package sapl

import (
	"strconv"
	"time"
)

type OpCode int

const (
	OpPanic OpCode = -1234567890
	OpAdd   OpCode = -1
	OpSub   OpCode = -2
	OpMul   OpCode = -3
	OpDiv   OpCode = -4
	OpMod   OpCode = -5
	OpEq    OpCode = -6
	OpLt    OpCode = -7
	OpGt    OpCode = -8
)

type CtxEval struct {
	Tracer func(Expr, []Expr) func(Expr) Expr

	Stats struct {
		MaxStack    int
		NumSteps    int
		NumRebuilds int
		NumCalls    int
		TimeTaken   time.Duration
	}
}

func (me *CtxEval) String() string {
	return me.Stats.TimeTaken.String() + "\tMaxStack=" + strconv.Itoa(me.Stats.MaxStack) + "\tNumSteps=" + strconv.Itoa(me.Stats.NumSteps) + "\tNumRebuilds=" + strconv.Itoa(me.Stats.NumRebuilds) + "\tNumCalls=" + strconv.Itoa(me.Stats.NumCalls)
}

func (me Prog) Eval(ctx *CtxEval, expr Expr) (ret Expr, retList []Expr) {
	if retList = make([]Expr, 0, 128); ctx.Tracer == nil {
		ret := func(it Expr) Expr { return it }
		ctx.Tracer = func(Expr, []Expr) func(Expr) Expr { return ret }
	}
	stack := make([]Expr, 0, 128)
	tstart := time.Now().UnixNano()
	ret = me.eval(expr, stack, ctx)
	retList = me.List(ctx, ret)
	ctx.Stats.TimeTaken = time.Duration(time.Now().UnixNano() - tstart)
	return
}

func (me Prog) eval(expr Expr, stack []Expr, ctx *CtxEval) Expr {
	if ctx.Stats.NumSteps++; len(stack) > ctx.Stats.MaxStack {
		ctx.Stats.MaxStack = len(stack)
	}
	ret := ctx.Tracer(expr, stack)
	switch it := expr.(type) {
	case ExprAppl:
		return ret(me.eval(it.Callee, append(stack, it.Arg), ctx))
	case ExprFnRef:
		numargs, isopcode := 2, (it < 0)
		if !isopcode {
			numargs = me[it].NumArgs
		}
		if len(stack) < numargs { // not enough args on stack: a partial-application aka closure
			ctx.Stats.NumRebuilds++
			for i := len(stack) - 1; i >= 0; i-- {
				expr = ExprAppl{expr, stack[i]}
			}
			return ret(expr)
		} else if isopcode {
			opcode := OpCode(it)
			lhs, rhs := me.eval(stack[len(stack)-1], stack, ctx).(ExprNum), me.eval(stack[len(stack)-2], stack, ctx).(ExprNum)
			stack = stack[:len(stack)-2]
			switch opcode {
			case OpAdd:
				return ret(lhs + rhs)
			case OpSub:
				return ret(lhs - rhs)
			case OpMul:
				return ret(lhs * rhs)
			case OpDiv:
				return ret(lhs / rhs)
			case OpMod:
				return ret(lhs % rhs)
			case OpEq, OpGt, OpLt:
				if (opcode == OpEq && lhs == rhs) || (opcode == OpLt && lhs < rhs) || (opcode == OpGt && lhs > rhs) {
					it, numargs = 1, 2
				} else {
					it, numargs = 2, 2
				}
			default:
				panic(stack)
			}
		}
		ctx.Stats.NumCalls++
		return ret(me.eval(argRefsResolvedToCurrentStackEntries(me[it].Expr, stack), stack[:len(stack)-numargs], ctx))
	}
	return ret(expr)
}

func argRefsResolvedToCurrentStackEntries(expr Expr, stack []Expr) Expr {
	switch it := expr.(type) {
	case ExprAppl:
		return ExprAppl{argRefsResolvedToCurrentStackEntries(it.Callee, stack), argRefsResolvedToCurrentStackEntries(it.Arg, stack)}
	case ExprArgRef:
		return stack[len(stack)+int(it)]
	}
	return expr
}

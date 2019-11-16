package tinylam

import (
	"strconv"
)

type instr int

const ( // if wanting to re-arrange: `Eval` requires that all arith-instrs do precede all compare-instrs (of which EQ must be the first)
	_ instr = iota
	instrADD
	instrMUL
	instrSUB
	instrDIV
	instrMOD
	instrMSG
	instrERR
	instrEQ
	instrGT
	instrLT
)

var (
	instrs     = map[string]instr{"ADD": instrADD, "MUL": instrMUL, "SUB": instrSUB, "DIV": instrDIV, "MOD": instrMOD, "MSG": instrMSG, "ERR": instrERR, "EQ": instrEQ, "GT": instrGT, "LT": instrLT}
	instrNames = []string{"?!?", "ADD", "MUL", "SUB", "DIV", "MOD", "MSG", "ERR", "EQ", "GT", "LT"}
)

type Expr interface {
	locInfo() *nodeLocInfo
	replaceName(string, string) int
	rewriteName(string, Expr) Expr
	String() string
}

type ExprLitNum struct {
	*nodeLocInfo
	NumVal int
}

func (me *ExprLitNum) rewriteName(string, Expr) Expr  { return me }
func (me *ExprLitNum) replaceName(string, string) int { return 0 }
func (me *ExprLitNum) String() string                 { return strconv.FormatInt(int64(me.NumVal), 10) }

type ExprName struct {
	*nodeLocInfo
	NameVal    string
	idxOrInstr int // if <0 then De Bruijn index, if >0 then instrCode
}

func (me *ExprName) rewriteName(name string, with Expr) Expr {
	if me.NameVal == name {
		return with
	} else if me.idxOrInstr < 0 {
		me.idxOrInstr = 0
	}
	return me
}
func (me *ExprName) replaceName(nameOld string, nameNew string) (didReplace int) {
	if me.NameVal == nameOld { // even if nameOld==nameNew, by design: as we use it also to check "refersTo" by doing `replaceName("foo", "foo")`
		didReplace, me.NameVal = 1, nameNew
	}
	return
}
func (me *ExprName) String() string { return me.NameVal + ":" + strconv.Itoa(me.idxOrInstr) }

type ExprCall struct {
	*nodeLocInfo
	Callee  Expr
	CallArg Expr
}

func (me *ExprCall) rewriteName(name string, with Expr) Expr {
	me.Callee, me.CallArg = me.Callee.rewriteName(name, with), me.CallArg.rewriteName(name, with)
	return me
}
func (me *ExprCall) replaceName(nameOld string, nameNew string) int {
	return me.Callee.replaceName(nameOld, nameNew) + me.CallArg.replaceName(nameOld, nameNew)
}
func (me *ExprCall) String() string {
	return "(" + me.Callee.String() + " " + me.CallArg.String() + ")"
}

type ExprFunc struct {
	*nodeLocInfo
	ArgName string
	Body    Expr

	numArgUses int
}

func (me *ExprFunc) rewriteName(name string, with Expr) Expr {
	me.Body = me.Body.rewriteName(name, with)
	return me
}
func (me *ExprFunc) replaceName(old string, new string) int { return me.Body.replaceName(old, new) }
func (me *ExprFunc) String() string                         { return "{ " + me.ArgName + " -> " + me.Body.String() + " }" }

type Value interface {
	isClosure() *valClosure
	isNum() *valNum
	force() Value
	String() string
}

type valEq interface{ eq(Value) bool }

type valNum int

func (me valNum) eq(cmp Value) bool      { it := cmp.isNum(); return it != nil && me == *it }
func (me valNum) force() Value           { return me }
func (me valNum) isClosure() *valClosure { return nil }
func (me valNum) isNum() *valNum         { return &me }
func (me valNum) String() string         { return strconv.FormatInt(int64(me), 10) }

type valClosure struct {
	env     Values
	body    Expr
	instr   instr
	argDrop bool
}

func (me *valClosure) force() Value           { return me }
func (me *valClosure) isClosure() *valClosure { return me }
func (me *valClosure) isNum() *valNum         { return nil }
func (me *valClosure) String() (r string) {
	if r = "closureEnv#" + strconv.Itoa(len(me.env)) + "#"; me.body != nil {
		r += me.body.String()
	} else if instr := int(me.instr); instr != 0 && instr < len(instrNames) {
		if instr < 0 {
			instr = 0 - instr
		}
		r += instrNames[instr]
	} else {
		r += "?!NEWBUG!?"
	}
	return
}

type valThunk struct{ val interface{} }

func (me *valThunk) eq(cmp Value) bool {
	self, _ := me.force().(valEq)
	return self != nil && self.eq(cmp.force())
}
func (me *valThunk) isClosure() *valClosure { return me.force().isClosure() }
func (me *valThunk) isNum() *valNum         { return me.force().isNum() }
func (me *valThunk) String() string         { return me.force().String() }
func (me *valThunk) force() (r Value) {
	if r, _ = me.val.(Value); r == nil {
		r = (me.val.(func(*valThunk) Value)(me)).force()
	}
	return
}

type Values []Value

func (me Values) shallowCopy() Values { return append(make(Values, 0, len(me)), me...) }

func (me *Prog) Eval(expr Expr, env Values) Value {
	me.NumEvalSteps++
	switch it := expr.(type) {
	case *ExprLitNum:
		return valNum(it.NumVal)
	case *ExprFunc:
		return &valClosure{body: it.Body, env: env.shallowCopy(), argDrop: it.numArgUses == 0}
	case *ExprName:
		if it.idxOrInstr > 0 { // it's never 0 thanks to prior & completed `Prog.preResolveNames`
			return &valClosure{instr: instr(-it.idxOrInstr), env: env.shallowCopy()}
		} else if it.idxOrInstr == 0 {
			panic(it.locStr() + "NEWLY INTRODUCED INTERPRETER BUG: " + it.String())
		}
		return env[len(env)+it.idxOrInstr]
	case *ExprCall:
		callee := me.Eval(it.Callee, env)
		closure := callee.isClosure()
		if closure == nil {
			panic(it.locStr() + "not callable: `" + it.Callee.String() + "`, which equates to `" + callee.String() + "`, in: " + it.String())
		}
		var argval Value
		if closure.argDrop {
			argval = nil // dummy val for the arg that WILL be discarded given the selector we're in
		} else if me.LazyEval {
			argval = &valThunk{val: func(set *valThunk) Value { ret := me.Eval(it.CallArg, env); set.val = ret; return ret }}
		} else {
			argval = me.Eval(it.CallArg, env)
		}
		if closure.instr < 0 {
			closure.instr, closure.env = -closure.instr, append(closure.env, argval) // return &valClosure{instr: -closure.instr, env: append(closure.env.Copy(), argval)}
			if closure.instr == instrERR {
				if argval = me.Value(argval); me.OnInstrMSG != nil {
					me.OnInstrMSG(it.locStr(), argval)
				}
				panic(argval)
			}
			return closure
		} else if closure.instr > 0 {
			lhs, rhs := closure.env[len(closure.env)-1], argval
			lnum, rnum := lhs.isNum(), rhs.isNum()
			if closure.instr == instrMSG {
				if me.OnInstrMSG != nil {
					me.OnInstrMSG(string(me.Value(lhs).(valFinalBytes)), rhs)
				}
				return argval
			}
			if closure.instr < instrEQ && lnum != nil && rnum != nil {
				return closure.instr.callCalc(it.locInfo(), *lnum, *rnum)
			}
			var retbool *ExprFunc
			if closure.instr >= instrEQ && lnum != nil && rnum != nil {
				retbool = me.newBool(closure.instr.callCmp(it.locInfo(), *lnum, *rnum))
			} else if closure.instr == instrEQ {
				if eq, _ := me.value(lhs).(valEq); eq != nil {
					retbool = me.newBool(eq.eq(me.value(rhs)))
				}
			}
			if retbool == nil {
				panic(it.locStr() + "invalid operands for '" + instrNames[closure.instr] + "' instruction: `" + lhs.String() + "` and `" + rhs.String() + "`, in: `" + it.String() + "`")
			}
			return me.Eval(retbool, env)
		}
		return me.Eval(closure.body, append(closure.env, argval))
	}
	panic(expr)
}

func (me *Prog) newBool(b bool) (exprTrueOrFalse *ExprFunc) {
	if exprTrueOrFalse = me.exprBoolFalse; b {
		exprTrueOrFalse = me.exprBoolTrue
	}
	return
}

func (me instr) callCalc(loc *nodeLocInfo, lhs valNum, rhs valNum) valNum {
	switch me {
	case instrADD:
		return lhs + rhs
	case instrSUB:
		return lhs - rhs
	case instrMUL:
		return lhs * rhs
	case instrDIV:
		return lhs / rhs
	case instrMOD:
		return lhs % rhs
	}
	panic(loc.locStr() + "unknown calc-instruction code: " + strconv.Itoa(int(me)))
}

func (me instr) callCmp(loc *nodeLocInfo, lhs valNum, rhs valNum) bool {
	switch me {
	case instrEQ:
		return (lhs == rhs)
	case instrGT:
		return (lhs > rhs)
	case instrLT:
		return (lhs < rhs)
	}
	panic(loc.locStr() + "unknown compare-instruction code: " + strconv.Itoa(int(me)))
}
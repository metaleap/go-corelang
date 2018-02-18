package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type opCode int

const (
	_ opCode = iota
	OP_LIT
	OP_EXEC
	OP_LOAD
	OP_STORE
	OP_CALL
	OP_INCR
	OP_JUMP
	OP_JUMPCOND
)

type instr struct {
	Op opCode
	A  int
	L  int
}

var opExit = instr{Op: OP_JUMP}

func main() {
	machine := interp{}

	machine.runDemo("(123×456)÷789", "71", []instr{
		{Op: OP_LIT, A: 123},
		{Op: OP_LIT, A: 456},
		{Op: OP_EXEC, A: EXEC_AR_MUL},
		{Op: OP_LIT, A: 789},
		{Op: OP_EXEC, A: EXEC_AR_DIV},
		opExit,
	})

	machine.runDemo("987×(654-321)", "328671", []instr{
		{Op: OP_LIT, A: 987},
		{Op: OP_LIT, A: 654},
		{Op: OP_LIT, A: 321},
		{Op: OP_EXEC, A: EXEC_AR_SUB},
		{Op: OP_EXEC, A: EXEC_AR_MUL},
		opExit,
	})

	readln, write := bufio.NewScanner(os.Stdin), os.Stdout.WriteString
	write("\n\nEnter one of the following function names, followed by 1 space and 1 number:\n")
	write("· negodd — negates the number if it is odd\n")
	write("· negeven — negates the number if it is even\n")
	write("· fac — computes the number's factorial\n")
	write("· q — quits\n\n")
	for readln.Scan() {
		if ln := strings.TrimSpace(readln.Text()); ln != "" {
			if ln == "q" {
				return
			}
			isnodd, isnev, isfac := strings.HasPrefix(ln, "negodd"), strings.HasPrefix(ln, "negeven"), strings.HasPrefix(ln, "fac")
			if i := strings.IndexRune(ln, ' '); i > 0 && (isnodd || isnev || isfac) {
				num, err := strconv.ParseInt(ln[i+1:], 0, 64)
				if err != nil {
					println(err.Error())
				} else {
					arg := int(num)

					var result int
					var timetaken time.Duration
					if isnodd {
						result, timetaken = machine.runNegIf(1, arg)
					} else if isnev {
						result, timetaken = machine.runNegIf(0, arg)
					} else {
						result, timetaken = machine.runFac(arg)
					}
					println(timetaken.String())
					println(result)
				}
			}
		}
	}
}

func (me *interp) runDemo(descr string, expectedResult string, programCode []instr) {
	println("Calcing " + descr + ".. — should be: " + expectedResult)
	me.code = programCode
	println(me.run())
}

func (me *interp) runNegIf(isNegIfOdd int, num int) (result int, timeTaken time.Duration) {
	if me.code = codeNegIfEven; isNegIfOdd != 0 {
		me.code = codeNegIfOdd
	}
	me.code[0].A, me.code[3].A, me.code[5+isNegIfOdd].A = num, num, num

	timestarted := time.Now()
	result = me.run()
	timeTaken = time.Now().Sub(timestarted)
	return
}

var codeNegIfEven = []instr{
	{Op: OP_LIT},
	{Op: OP_EXEC, A: EXEC_ODD},
	{Op: OP_JUMPCOND, A: 5},
	{Op: OP_LIT},
	opExit,
	{Op: OP_LIT},
	{Op: OP_EXEC, A: EXEC_NEG},
	opExit,
}

var codeNegIfOdd = []instr{
	{Op: OP_LIT},
	{Op: OP_EXEC, A: EXEC_ODD},
	{Op: OP_JUMPCOND, A: 6},
	{Op: OP_LIT},
	{Op: OP_EXEC, A: EXEC_NEG},
	opExit,
	{Op: OP_LIT},
	opExit,
}

func (me *interp) runFac(num int) (result int, timeTaken time.Duration) {
	me.code = codeFactorialLoop // codeFacRecursionFail
	me.code[1].A = num
	me.code[2].A = len(me.code) - 1
	// me.code[0].A = num
	// me.code[3].A = len(me.code) - 2

	timestarted := time.Now()
	result = me.run()
	timeTaken = time.Now().Sub(timestarted)
	return
}

var codeFactorialLoop = []instr{ // r := 1; for n>0 { r=r*n ; n = n-1 }
	{Op: OP_LIT, A: 1}, //r=1, t1
	{Op: OP_LIT},       //n, t2

	{Op: OP_JUMPCOND},             // n>0, t1
	{Op: OP_INCR, A: 1},           // restore n, t2
	{Op: OP_STORE, A: 77},         // stow away n, t1
	{Op: OP_INCR, A: 1},           // but keep it,t2
	{Op: OP_EXEC, A: EXEC_AR_MUL}, // r=r*n, t1
	{Op: OP_LOAD, A: 77},          // restore n, t2
	{Op: OP_LIT, A: 1},            // 1, t3
	{Op: OP_EXEC, A: EXEC_AR_SUB}, // n=n-1, t2
	{Op: OP_JUMP, A: 2},           // t2

	opExit,
}

var codeFactorialRecurse = []instr{ // 	(n==0) ? 1 : (n * fac(n-1))
	{Op: OP_LIT},
	{Op: OP_CALL, A: 3, L: 1},
	opExit,

	{Op: OP_JUMPCOND},

	{Op: OP_LOAD},                 // n
	{Op: OP_LIT, A: 1},            // 1
	{Op: OP_EXEC, A: EXEC_AR_SUB}, // n-1
	{Op: OP_LOAD, L: 1},           // n
	{Op: OP_CALL, A: 3, L: 1},     // fac(n-1)
	{Op: OP_EXEC, A: EXEC_AR_MUL}, // n*fac(n-1)
	{Op: OP_EXEC, A: EXEC_RET},

	{Op: OP_LIT, A: 1},
	{Op: OP_EXEC, A: EXEC_RET},
}

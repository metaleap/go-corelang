package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type multiplicator struct {
	Operand1     int64
	Operand2     int64
	Jobber       int64
	RunningTotal int64
}

func main() {
	mul, readln, write := &multiplicator{}, bufio.NewScanner(os.Stdin), os.Stdout.WriteString
	for {
		if write("Keep entering 2 ints (separated by 1 space) to have them (horribly inefficiently) multiplied by a state transition machine via mere incr1/decr1 operations:\n"); !readln.Scan() {
			return
		} else if operands := strings.Split(strings.TrimSpace(readln.Text()), " "); len(operands) != 2 {
			write("try again\n")
		} else {
			operand1, _ := strconv.ParseInt(operands[0], 0, 64)
			operand2, _ := strconv.ParseInt(operands[1], 0, 64)
			result := mul.eval(operand1, operand2)
			write(strconv.FormatInt(result, 10) + "\n")
		}
	}
}

func (me *multiplicator) eval(op1 int64, op2 int64) int64 {
	for me.init(op1, op2); !me.finalState(); me.step() {
		// nothing else to do here while we step
	}
	return me.RunningTotal
}

func (me *multiplicator) init(op1 int64, op2 int64) {
	me.Operand1, me.Operand2, me.Jobber, me.RunningTotal = op1, op2, 0, 0
}

func (me *multiplicator) finalState() bool {
	return me.Operand2 == 0 && me.Jobber == 0
}

func (me *multiplicator) step() {
	if me.Jobber == 0 {
		me.Jobber, me.Operand2 = me.Operand1, me.Operand2-1
	} else {
		me.Jobber, me.RunningTotal = me.Jobber-1, me.RunningTotal+1
	}
}
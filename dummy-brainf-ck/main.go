package main

// inspired by thread: https://www.reddit.com/r/golang/comments/8ym8lf/why_is_this_simple_benchmark_3_times_faster_in/

import (
	"io/ioutil"
	"os"
)

const srcAlphabetReverse = `
>++[<+++++++++++++>-]<[[>+>+<<-]>[<+>-]++++++++
[>++++++++<-]>.[-]<<>++++++++++[>++++++++++[>++
++++++++[>++++++++++[>++++++++++[>++++++++++[>+
+++++++++[-]<-]<-]<-]<-]<-]<-]<-]++++++++++.
`

const (
	INC = iota
	MOVE
	PRINT
	LOOP
)

type code = []instr

type instr struct {
	opCode int
	val    int
	loop   code
}

var machine struct {
	tape []int
	pos  int
}

func main() {
	src := []byte(srcAlphabetReverse)
	if len(os.Args) > 1 {
		if filesrc, err := ioutil.ReadFile(os.Args[1]); err != nil {
			panic(err)
		} else {
			src = filesrc
		}
	}
	prog := parse(src)

	machine.tape = make([]int, 1, 16)
	run(prog)
}

func parse(src []byte) code {
	opinc, opdec, opmovr, opmovl, opprint :=
		instr{opCode: INC, val: 1}, instr{opCode: INC, val: -1}, instr{opCode: MOVE, val: 1}, instr{opCode: MOVE, val: -1}, instr{opCode: PRINT}
	stack := make([]code, 1, 16)
	stack[0] = make(code, 0, len(src))

	var cur int
	for pos := range src {
		var op instr
		switch src[pos] {
		case '+':
			op = opinc
		case '-':
			op = opdec
		case '>':
			op = opmovr
		case '<':
			op = opmovl
		case '.':
			op = opprint
		case ']':
			// if cur > 0 {
			stack, op = stack[:cur], instr{opCode: LOOP, loop: stack[cur]}
			cur--
			// } else { panic("source error: unmatched closing bracket") }
		case '[':
			stack = append(stack, make(code, 0, len(src)-pos))
			cur++
			continue
		default:
			continue
		}
		stack[cur] = append(stack[cur], op)
	}
	return stack[0]
}

func run(prog code) {
	for i := range prog {
		switch prog[i].opCode {
		case INC:
			machine.tape[machine.pos] += prog[i].val
		case MOVE:
			machine.pos += prog[i].val
			if overshoot := (machine.pos - len(machine.tape)); overshoot > -1 {
				machine.tape = append(machine.tape, make([]int, overshoot+1)...)
			}
		case PRINT:
			os.Stdout.WriteString(string(machine.tape[machine.pos]))
		case LOOP:
			for machine.tape[machine.pos] > 0 {
				run(prog[i].loop)
			}
		}
	}
}

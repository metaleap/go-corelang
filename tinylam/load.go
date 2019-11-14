package tinylam

import (
	"bytes"
	"strconv"
	"strings"
)

const (
	StdModuleName               = "std"
	StdRequiredDefs_true        = StdModuleName + "." + "True"
	StdRequiredDefs_false       = StdModuleName + "." + "False"
	StdRequiredDefs_listCons    = StdModuleName + "." + "Cons"
	StdRequiredDefs_listNil     = StdModuleName + "." + "Nil"
	StdRequiredDefs_listIsNil   = StdModuleName + "." + "__tlListIsNil"
	StdRequiredDefs_listIsntNil = StdModuleName + "." + "__tlListIsntNil"
)

type ctxParse struct {
	prog      *Prog
	srcs      map[string][]byte
	counter   int
	curModule struct{ name string }
	curTopDef struct {
		bracketsParens  map[string]string
		bracketsCurlies map[string]string
		bracketsSquares map[string]string
	}
}

type nodeLocInfo struct {
	srcLocModuleName string
	srcLocTopDefName string
	srcLocLineNr     int
}

func (me *nodeLocInfo) locInfo() *nodeLocInfo { return me }
func (me *nodeLocInfo) locStr() string {
	if me == nil {
		return ""
	}
	return "in '" + me.srcLocModuleName + "." + me.srcLocTopDefName + "', line " + strconv.Itoa(me.srcLocLineNr) + ": "
}

func (me *Prog) ParseModules(modules map[string][]byte) {
	ctx := ctxParse{prog: me, srcs: modules}
	ctx.curTopDef.bracketsParens, ctx.curTopDef.bracketsCurlies, ctx.curTopDef.bracketsSquares = make(map[string]string, 16), make(map[string]string, 2), make(map[string]string, 4) // reset every top-def, potentially needed earlier for type-spec top-defs
	me.NumEvalSteps, me.TopDefs = 0, map[string]Expr{}

	for modulename, modulesrc := range modules {
		ctx.curModule.name = modulename
		module := ctx.parseModule(ctx.rewriteStrLitsToIntLists(modulesrc, modulename))
		for topdefname, topdefbody := range module {
			me.TopDefs[modulename+"."+topdefname] = ctx.populateNames(topdefbody, make(map[string]int, 16), module, topdefname)
		}
	}
	me.exprBoolTrue, me.exprBoolFalse = me.TopDefs[StdRequiredDefs_true].(*ExprFunc), me.TopDefs[StdRequiredDefs_false].(*ExprFunc)
	me.exprBoolTrueBodyBody, me.exprListConsBodyBodyBody = me.exprBoolTrue.Body.(*ExprFunc).Body, me.TopDefs[StdRequiredDefs_listCons].(*ExprFunc).Body.(*ExprFunc).Body.(*ExprFunc).Body
	me.TopDefs[StdModuleName+".!"], me.TopDefs[StdModuleName+".?"] = me.TopDefs[StdRequiredDefs_listIsNil], me.TopDefs[StdRequiredDefs_listIsntNil]
	for instrname, instrcode := range instrs {
		me.TopDefs[StdModuleName+".//op"+instrname] = &ExprFunc{nil, "//" + instrname, &ExprCall{nil, &ExprName{nil, instrname, int(instrcode)}, &ExprName{nil, "//" + instrname, -1}}}
	}
	for topdefqname, topdefbody := range me.TopDefs {
		me.TopDefs[topdefqname] = me.preResolveExprs(topdefbody, topdefqname, topdefbody)
	}
}

func (me *ctxParse) parseModule(src string) map[string]Expr {
	lines, module := strings.Split(src, "\n"), make(map[string]Expr, 32)
	if strings.IndexByte(src, '|') > 0 {
		for l, i := len(lines), 0; i < l; i++ {
			if ln := lines[i]; len(ln) > 0 && ln[0] >= 'A' && ln[0] <= 'Z' {
				if idx := strings.Index(ln, ":="); idx > 0 {
					if tparts, cparts := strings.Fields(ln[:idx]), strings.Split(me.extractBrackets(nil, strings.TrimSpace(ln[idx+2:]), ln, true), " | "); len(tparts) > 0 && len(cparts) > 0 && len(cparts[0]) > 0 {
						lines[i] = "//" + lines[i]
						for _, cpart := range cparts {
							str := cpart + " :="
							for _, ctorstr := range cparts {
								ctorstr += " "
								str += " __caseOf" + ctorstr[:strings.IndexByte(ctorstr, ' ')]
							}
							str += " -> __caseOf" + cpart
							println(str)
							lines = append(lines, "", str, "")
						}
					}
				}
			}
		}
	}
	for idx, last, i := 0, len(lines), len(lines)-1; i >= 0; i-- {
		if idx = strings.Index(lines[i], "//"); idx >= 0 {
			lines[i] = lines[i][:idx]
		}
		if nonempty := (len(lines[i]) > 0); i == 0 || (nonempty && lines[i][0] != ' ' && lines[i][0] != '\t') {
			if topdefname, topdefbody, firstln := me.parseTopDef(lines, i, last); topdefname != "" && topdefbody != nil {
				if module[topdefname] != nil || topdefname == "_" || strings.IndexByte(topdefname, '.') >= 0 || (len(topdefname) > 1 && (topdefname[0] == '?' || topdefname[0] == '!')) {
					panic("in '" + me.curModule.name + "', line " + strconv.Itoa(i+1) + ": illegal or duplicate global def name '" + topdefname + "' in:\n" + firstln)
				}
				module[topdefname] = topdefbody
			}
			last = i
		}
	}
	return module
}

func (me *ctxParse) parseTopDef(lines []string, idxStart int, idxEnd int) (topDefName string, topDefBody Expr, firstLn string) {
	topDefName, me.curTopDef.bracketsParens, me.curTopDef.bracketsCurlies, me.curTopDef.bracketsSquares = "?", make(map[string]string, 16), make(map[string]string, 2), make(map[string]string, 4)
	var topdefargs []string
	for i, ln := range lines[idxStart:idxEnd] {
		if ln = strings.TrimSpace(ln); ln != "" {
			lnorig, loc := ln, &nodeLocInfo{me.curModule.name, topDefName, 1 + i + idxStart}
			for idx := strings.IndexByte(ln, '\''); idx >= 0 && (idx+3) <= len(ln); idx = strings.IndexByte(ln, '\'') {
				ln = ln[:idx] + " " + strconv.FormatUint(uint64(ln[idx+1]), 10) + " " + ln[idx+3:]
			}
			ln = me.extractBrackets(loc, ln, lnorig, false)
			if idx := strings.Index(ln, ":="); idx < 0 {
				panic(loc.locStr() + "expected ':=' in:\n" + lnorig)
			} else if sl, sr := strings.TrimSpace(ln[:idx]), strings.TrimSpace(ln[idx+2:]); sl == "" || sr == "" {
				panic(loc.locStr() + "expected '<name/s> := <expr>' in:\n" + lnorig)
			} else if lhs, rhs := strings.Fields(sl), strings.Fields(sr); firstLn == "" {
				firstLn, topDefName, loc.srcLocTopDefName, topdefargs = lnorig, lhs[0], lhs[0], lhs[1:]
				topDefBody = me.parseExpr(rhs, lnorig, loc)
			} else if localname := lhs[0]; localname == "_" || strings.IndexByte(localname, '.') >= 0 || (len(localname) > 1 && (localname[0] == '!' || localname[0] == '?')) {
				panic(loc.locStr() + "illegal  local def name '" + localname + "' in:\n" + lnorig)
			} else {
				localbody := me.hoistArgs(me.parseExpr(rhs, lnorig, loc), lhs[1:])
				if localbody.replaceName(localname, "//recur3//"+localname) {
					localbody = me.rewriteForRecursion(localname, localbody, "recur")
				}
				topDefBody = &ExprCall{loc, &ExprFunc{loc, localname, topDefBody}, localbody}
			}
		}
	}
	topDefBody = me.hoistArgs(topDefBody, topdefargs)
	if topDefBody != nil && topDefBody.replaceName(topDefName, topDefName) /* aka "refers to"*/ {
		topDefBody = me.rewriteForRecursion(topDefName, topDefBody, "Recur")
	}
	return
}

func (me *ctxParse) rewriteForRecursion(defName string, defBody Expr, dynNamePref string) Expr {
	return &ExprCall{defBody.locInfo(), &ExprFunc{defBody.locInfo(), "//" + dynNamePref + "1//" + defName, &ExprCall{defBody.locInfo(), &ExprName{defBody.locInfo(), "//" + dynNamePref + "1//" + defName, 0}, &ExprName{defBody.locInfo(), "//" + dynNamePref + "1//" + defName, 0}}}, &ExprFunc{defBody.locInfo(), "//" + dynNamePref + "2//" + defName, defBody}}
}

func (me *ctxParse) parseExpr(toks []string, locHintLn string, locInfo *nodeLocInfo) (expr Expr) {
	if len(toks) == 0 {
		expr = &ExprCall{locInfo, &ExprName{locInfo, "ERR", int(instrERR)}, me.prog.newStr(locInfo, locInfo.locStr()+"abyss")}
	} else if tok, islambda, lamsplit := toks[0], 0, 0; len(toks) > 1 {
		me.counter++
		for i := range toks {
			if lamsplit == 0 && toks[i] == "->" {
				lamsplit = i
				break
			} else if l := len(toks[i]); lamsplit == 0 && toks[i][0] == '_' && toks[i] == strings.Repeat("_", l) {
				if toks[i] = "//lam//" + strconv.Itoa(l) + "//" + strconv.Itoa(me.counter); l > islambda {
					islambda = l
				}
			}
		}
		if args := toks[:lamsplit]; lamsplit > 0 {
			for i, tok := range toks[:lamsplit] {
				if tok[0] == '/' {
					me.counter, toks[i] = me.counter+1, "//"+strconv.Itoa(i)+"//"+strconv.Itoa(me.counter)
				}
			}
			expr = me.hoistArgs(me.parseExpr(toks[lamsplit+1:], locHintLn, locInfo), args)
		} else if args = make([]string, islambda); islambda > 0 {
			for i := range args {
				args[i] = "//lam//" + strconv.Itoa(i+1) + "//" + strconv.Itoa(me.counter)
			}
			expr = me.hoistArgs(me.parseExpr(toks, locHintLn, locInfo), args)
		} else {
			expr = &ExprCall{locInfo, me.parseExpr(toks[:len(toks)-1], locHintLn, locInfo), me.parseExpr(toks[len(toks)-1:], locHintLn, locInfo)}
		}
	} else if isnum, isneg := (tok[0] >= '0' && tok[0] <= '9'), tok[0] == '-' && len(tok) > 1; isnum || (isneg && tok[1] >= '0' && tok[1] <= '9') {
		if numint, err := strconv.ParseInt(tok, 0, 0); err != nil {
			panic(locInfo.locStr() + err.Error() + " in:\n" + locHintLn)
		} else {
			expr = &ExprLitNum{locInfo, int(numint)}
		}
	} else if subexpr, ok := me.curTopDef.bracketsParens[tok]; ok {
		expr = me.parseExpr(strings.Fields(subexpr), locHintLn, locInfo)
	} else if subexpr, ok = me.curTopDef.bracketsSquares[tok]; ok {
		expr = &ExprName{locInfo, StdRequiredDefs_listNil, 0}
		items := strings.Fields(subexpr)
		for i := len(items) - 1; i >= 0; i-- {
			expr = &ExprCall{locInfo, &ExprCall{locInfo, &ExprName{locInfo, StdRequiredDefs_listCons, 0}, me.parseExpr(items[i:i+1], locHintLn, locInfo)}, expr}
		}
	} else if subexpr, ok = me.curTopDef.bracketsCurlies[tok]; ok {
		if items := strings.Fields(subexpr); len(items) == 0 {
			expr = &ExprName{locInfo, StdRequiredDefs_listCons, 0}
		} else if len(items) == 1 {
			expr = &ExprCall{locInfo, &ExprName{locInfo, StdRequiredDefs_listCons, 0}, me.parseExpr(items, locHintLn, locInfo)}
		} else {
			expr = me.parseExpr(items[len(items)-1:], locHintLn, locInfo)
			for i := len(items) - 2; i >= 0; i-- {
				expr = &ExprCall{locInfo, &ExprCall{locInfo, &ExprName{locInfo, StdRequiredDefs_listCons, 0}, me.parseExpr(items[i:i+1], locHintLn, locInfo)}, expr}
			}
		}
	} else {
		expr = &ExprName{locInfo, tok, int(instrs[tok])}
	}
	return
}

func (me *ctxParse) rewriteStrLitsToIntLists(src []byte, name string) string {
	if bytes.IndexByte(src, 0) >= 0 {
		panic("NUL char in module source: " + name)
	}
	src = bytes.ReplaceAll(src, []byte{'\\', '"'}, []byte{0})
	for idx := bytes.IndexByte(src, '"'); idx > 0; idx = bytes.IndexByte(src, '"') {
		if pos := bytes.IndexByte(src[idx+1:], '"'); pos < 0 {
			panic("in '" + me.curModule.name + "': non-terminated string literal: " + string(src[idx:]))
		} else {
			src[idx], src[idx+1+pos] = '[', ']'
			pref, suff, inner := src[:idx+1], src[idx+1+pos:], make([]byte, 0, 4*(1+pos))
			for i := idx + 1; i < idx+1+pos; i++ {
				b := src[i]
				if b == 0 {
					b = '"'
				}
				inner = append(append(inner, strconv.FormatUint(uint64(b), 10)...), ' ')
			}
			src = append(pref, append(inner, suff...)...)
		}
	}
	return string(bytes.ReplaceAll(src, []byte{0}, []byte{'\\', '"'}))
}

func (*ctxParse) hoistArgs(expr Expr, argNames []string) Expr {
	if expr != nil && len(argNames) != 0 {
		for i, argname := len(argNames)-1, ""; i >= 0; i-- {
			if argname = argNames[i]; argname == "_" {
				argname = strconv.Itoa(i)
			}
			expr = &ExprFunc{expr.locInfo(), argname, expr}
		}
	}
	return expr
}

func (me *ctxParse) populateNames(expr Expr, binders map[string]int, curModule map[string]Expr, locHintTopDefName string) Expr {
	const stdpref = StdModuleName + "."
	fixinstrval := func(expr Expr) {
		if name, _ := expr.(*ExprName); name != nil && name.idxOrInstr > 0 {
			name.NameVal, name.idxOrInstr = stdpref+"//op"+name.NameVal, 0
		}
	}
	switch it := expr.(type) {
	case *ExprCall:
		it.Callee = me.populateNames(it.Callee, binders, curModule, locHintTopDefName)
		it.CallArg = me.populateNames(it.CallArg, binders, curModule, locHintTopDefName)
		fixinstrval(it.CallArg)
	case *ExprName:
		if it.NameVal == locHintTopDefName {
			return me.populateNames(&ExprCall{it.locInfo(), &ExprName{it.locInfo(), "//Recur2//" + it.NameVal, 0}, &ExprName{it.locInfo(), "//Recur2//" + it.NameVal, 0}}, binders, curModule, locHintTopDefName)
		} else if strings.HasPrefix(it.NameVal, "//recur3//") {
			it.NameVal = it.NameVal[len("//recur3//"):]
			return me.populateNames(&ExprCall{it.locInfo(), &ExprName{it.locInfo(), "//recur2//" + it.NameVal, 0}, &ExprName{it.locInfo(), "//recur2//" + it.NameVal, 0}}, binders, curModule, locHintTopDefName)
		}
		if len(it.NameVal) > 1 && (it.NameVal[0] == '?' || it.NameVal[0] == '!') {
			return me.populateNames(&ExprCall{it.nodeLocInfo, &ExprName{it.nodeLocInfo, string(it.NameVal[0]), 0}, &ExprName{it.nodeLocInfo, it.NameVal[1:], 0}}, binders, curModule, locHintTopDefName)
		} else if posdot := strings.LastIndexByte(it.NameVal, '.'); posdot > 0 && nil == me.srcs[it.NameVal[:posdot]] && (nil == me.srcs[stdpref+it.NameVal[:posdot]] || 0 != binders[it.NameVal[:posdot]]) {
			dotpath := strings.Split(it.NameVal, ".") // desugar a.b.c into (c (b a))
			var ret Expr = &ExprName{it.nodeLocInfo, dotpath[0], 0}
			for i := 1; i < len(dotpath); i++ {
				ret = &ExprCall{it.nodeLocInfo, &ExprName{it.nodeLocInfo, dotpath[i], int(instrs[dotpath[i]])}, ret}
			}
			return me.populateNames(ret, binders, curModule, locHintTopDefName)
		} else if it.idxOrInstr == 0 && posdot < 0 { // neither a prim-instr-op-code, nor an already-qualified cross-module reference
			if it.idxOrInstr = binders[it.NameVal]; it.idxOrInstr > 0 {
				it.idxOrInstr = -it.idxOrInstr // mark as referring to a local / arg (De Bruijn index but negative)
			} else if _, topdefexists := curModule[it.NameVal]; topdefexists {
				it.NameVal = me.curModule.name + "." + it.NameVal // mark as referring to a global in the current module
			} else {
				it.NameVal = stdpref + it.NameVal // mark as referring to a global in std
			}
		}
	case *ExprFunc:
		if _, topdefexists := curModule[it.ArgName]; topdefexists || binders[it.ArgName] != 0 {
			panic("in '" + me.curModule.name + "." + locHintTopDefName + "', line " + strconv.Itoa(it.srcLocLineNr) + ": local name '" + it.ArgName + "' already taken: " + it.String())
		}
		for k, v := range binders {
			binders[k] = v + 1
		}
		binders[it.ArgName] = 1
		it.Body = me.populateNames(it.Body, binders, curModule, locHintTopDefName)
		if fixinstrval(it.Body); !it.Body.replaceName(it.ArgName, it.ArgName) {
			it.ArgName = ""
		}
		delete(binders, it.ArgName) // must delete, not just zero (because of our map-ranging incrs/decrs)
		for k, v := range binders {
			binders[k] = v - 1
		}
	}
	return expr
}

func (me *ctxParse) extractBrackets(loc *nodeLocInfo, ln string, lnOrig string, needLegalName bool) string {
	for str, b, idx, m, ip, ic, is := "", byte(0), 0, map[string]string(nil), strings.IndexByte(ln, ')'), strings.IndexByte(ln, '}'), strings.IndexByte(ln, ']'); ip > 0 || ic > 0 || is > 0; me.counter, ip, ic, is = me.counter+1, strings.IndexByte(ln, ')'), strings.IndexByte(ln, '}'), strings.IndexByte(ln, ']') {
		if p, c, s := ip > 0 && (ic <= 0 || ip < ic) && (is <= 0 || ip < is), ic > 0 && (ip <= 0 || ic < ip) && (is <= 0 || ic < is), is > 0 && (ic <= 0 || is < ic) && (ip <= 0 || is < ip); p {
			str, b, idx, m = "//bp//", '(', ip, me.curTopDef.bracketsParens
		} else if c {
			str, b, idx, m = "//bc//", '{', ic, me.curTopDef.bracketsCurlies
		} else if s {
			str, b, idx, m = "//bs//", '[', is, me.curTopDef.bracketsSquares
		}
		if name, pos := str+strconv.Itoa(me.counter), strings.LastIndexByte(ln[:idx], b); pos < 0 {
			panic(loc.locStr() + "missing opening '" + string(b) + "' bracket in:\n" + lnOrig)
		} else {
			if needLegalName {
				name = "_" + strings.Replace(ln[pos+1:idx], " ", "_", -1) + "_"
			}
			ln, m[name] = ln[:pos]+" "+name+" "+ln[idx+1:], ln[pos+1:idx]
		}
	}
	return ln
}

func (me *Prog) preResolveExprs(expr Expr, topDefQName string, topDefBody Expr) Expr {
	switch it := expr.(type) {
	case *ExprFunc:
		it.Body = me.preResolveExprs(it.Body, topDefQName, topDefBody)
	case *ExprCall:
		it.CallArg, it.Callee = me.preResolveExprs(it.CallArg, topDefQName, topDefBody), me.preResolveExprs(it.Callee, topDefQName, topDefBody)
		if call, _ := it.Callee.(*ExprCall); call != nil {
			if numlit, _ := it.CallArg.(*ExprLitNum); numlit != nil && call.ifConstNumArithOpInstrThenPreCalcInto(numlit, it) {
				return numlit
			}
		} else if fn, _ := it.Callee.(*ExprFunc); fn != nil {
			if fn.isIdentity() {
				return it.CallArg
				// } else if !fn.Body.replaceName(fn.ArgName, fn.ArgName) {
				// 	return fn.Body // branch commented because it never seems to occur for now, review later with more sizable code-base inputs
			}
		}
	case *ExprName:
		if it.idxOrInstr <= 0 {
			const stdpref = StdModuleName + "."
			topdefbody := me.TopDefs[it.NameVal]
			if topdefbody == nil {
				topdefbody = me.TopDefs[stdpref+it.NameVal]
			}
			if it.idxOrInstr == 0 {
				if topdefbody == nil && strings.HasPrefix(it.NameVal, stdpref) && strings.LastIndexByte(it.NameVal, '.') == len(StdModuleName) {
					needle := strings.TrimPrefix(it.NameVal, stdpref)
					for name, expr := range me.TopDefs {
						if strings.HasPrefix(name, stdpref) && strings.HasSuffix(name, "."+needle) {
							topdefbody = expr
							break
						}
					}
				}
				if topdefbody == nil {
					panic("in '" + topDefQName + "', line " + strconv.Itoa(it.srcLocLineNr) + ": name '" + it.NameVal + "' unresolvable in: " + topDefBody.String())
				} else if it.NameVal == topDefQName {
					panic(it.locStr() + "NEW BUG in `Prog.preResolveExprs` for top-level def '" + it.NameVal + "' recursion")
				} else if name, _ := topdefbody.(*ExprName); name != nil {
					return me.preResolveExprs(name, it.NameVal, name)
				} else {
					return topdefbody
				}
			} else if topdefbody != nil {
				panic("in '" + topDefQName + "', line " + strconv.Itoa(it.srcLocLineNr) + ": local name '" + it.NameVal + "' already taken (no shadowing allowed), referred to in: " + it.String())
			}
		}
	}
	return expr
}

func (me *ExprFunc) isIdentity() bool {
	name, ok := me.Body.(*ExprName)
	return ok && name.idxOrInstr == -1
}

func (me *ExprCall) ifConstNumArithOpInstrThenPreCalcInto(rhs *ExprLitNum, parent *ExprCall) (ok bool) {
	if name, _ := me.Callee.(*ExprName); name != nil && name.idxOrInstr > 0 {
		if lhs, _ := me.CallArg.(*ExprLitNum); lhs != nil {
			if instr := instr(name.idxOrInstr); instr < instrEQ {
				ok, rhs.nodeLocInfo, rhs.NumVal = true, parent.nodeLocInfo, int(instr.callCalc(parent.nodeLocInfo, valNum(lhs.NumVal), valNum(rhs.NumVal)))
			}
		}
	}
	return
}

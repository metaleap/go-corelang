
Token := TSepColon | TSepComma | TBracketSq _ | TBracketCy _ | TNumLit _ | TStrLit _ | TIdent _ | __TTmp _ _
Value := VNull | VBoolean _ | VInt _ | VStr _ | VArr (List _) | VObj (List (Pair k v))



stringified json  := json Value? VNull => casenull | VBoolean => casebool | VInt => caseint | VStr => casestr | VArr => casearr | VObj => caseobj
  casenull        := "null"
  casebool        := _ "true" "false"
  caseint         := num.toString
  casestr         := quoted
  casearr l       := concat (+> '[' (/> (l.each stringified) joinedVia ", ")) "]"
  caseobj l       := concat (+> '{' (/> (l.each ({k v} -> concat.** [stringified k, ": ", stringified v])) joinedVia ", ")) "}"
  quoted str      := concat (+> 34 str) [34]



tokenized src     := src ? .. => more
  more chr rest   := chr.simplecases Could? Have => +> _ rest.tokenized | None => others chr rest
  simplecases c   := (c.EQ ',') TSepComma.Have ((c.EQ '[') True.TBracketSq.Have ((c.EQ ']') False.TBracketSq.Have ((c.EQ '{') True.TBracketCy.Have ((c.EQ '}') False.TBracketCy.Have ((c.EQ ':') TSepColon.Have None)))))
  others c r      := c.checknum (casenum c r) (c.checkname (casename c r) (c.checkstr (casestr c r) (tokenized r)))
  casenum c rest  := (rest.brokenUpOn (_ -> checknum.not·)) (fst snd -> +> (TNumLit (c.+> fst)) (tokenized snd))
  casename c rest := (rest.brokenUpOn (_ -> checkname.not·)) (fst snd -> +> (TIdent (c.+> fst)) (tokenized snd))
  casestr c rest  := (rest.brokenUpOn (a c´ -> and c´.checkstr (a.last2 ? [] => True | .. => c1 c2 -> or (c2.-<.nEQ 92) (c1.EQ 92)))) (fst snd -> +> (TStrLit (concat (c.+> fst) [34])) (snd ? .. => _ r -> tokenized r))
  checknum chr    := and (chr.LT 58) (chr.GT 44)
  checkname chr   := and (chr.GT 96) (chr.LT 123)
  checkstr chr    := chr.EQ 34



parsed src        := src.tokenized.treeified ? [] => Nay "empty input" | .. => ··· handle parse
  treeified s     := (objs2tmp same.arrs2tmp) ((arrs2tmp same.objs2tmp) (s.whereNot (tok -> or tok.isTokenOfTSepComma tok.isTokenOfTSepColon)))
  arrs2tmp o toks := toks.rewritten (t -> t Token? TBracketSq => not | => False) (t -> t Token? TBracketSq => same | => False) (inner -> [__TTmp True inner.o])
  objs2tmp a toks := toks.rewritten (t -> t Token? TBracketCy => not | => False) (t -> t Token? TBracketCy => same | => False) (inner -> [__TTmp False inner.a])
  handle          := {tail ret} -> tail ? [] => ret | .. => _ _ -> Nay "trailing tokens remaining after completed expression"
  parse tok rest  := tok Token? TNumLit => pnum rest | TStrLit => pstr rest | TIdent => pident rest | __TTmp => pcomp rest parse | => {[], Nay "unexpected token"}

  pident r t      := { r, (t.EQ "null") (Yay VNull) ((t.EQ "true") (Yay True.VBoolean) ((t.EQ "false") (Yay False.VBoolean) (Nay t))) }
  pnum r t        := { r, t.parseSigned Result? Nay => Nay | Yay => ·· Yay VInt }
  pstr r t        := { r, t ? [] => Nay t | .. => quotemark rest -> Yay rest.sansLast.VStr } // caution, not handling escapes for now..
  pcomp r p arr l := (if (arr) parr (else pobj)) r p l
  parr rest p ts  := { rest, parryns (ts.each (t -> sndOfTuplePair (p t []))) }
    parryns yns   := chkfail yns (yns´ -> VArr yns´.iter)
  pobj rest p ts  := { rest, pobjyns (ts.each (t -> sndOfTuplePair (p t []))) }
    pobjyns yns   := chkfail yns (yns´ -> VObj yns´.iter.pairified)
  iter yns        := yns.each (_ same same)
  chkfail yns c   := (yns.findWhere isResultOfNay) Could? None => Yay yns.c | Have => same

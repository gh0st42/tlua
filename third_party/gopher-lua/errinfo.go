package lua

// The words of Lua 5.1's runtime errors. A type error names what it can of
// the value that was wrong, as ldebug.c does: it looks at the instruction that
// failed, finds the register the value was in, and works out from the code
// before it what the register held: a local, a global, a field of something, a
// method, an upvalue, a constant.
//
//	attempt to index global 'x' (a nil value)
//	attempt to call field 'f' (a nil value)
//	attempt to perform arithmetic on local 'n' (a string value)
//
// When it cannot tell, the error says only what the value is:
// "attempt to index a nil value".

// localName is the name of the local variable that is in register reg while
// the instruction at pc runs, or "".
func (p *FunctionProto) localName(reg, pc int) string {
	for _, l := range p.DbgLocals {
		if l.Reg == reg && l.StartPc <= pc && (!l.ended || pc <= l.EndPc) {
			return l.Name
		}
	}
	return ""
}

// setsReg is findsetreg: the last instruction before lastpc that changed
// register reg, or -1. A jump forward that does not go past lastpc is
// followed, as a conditional's code runs through it.
func (p *FunctionProto) setsReg(lastpc, reg int) int {
	set := -1
	for pc := 0; pc < lastpc; pc++ {
		inst := p.Code[pc]
		op := opGetOpCode(inst)
		a := opGetArgA(inst)
		switch op {
		case OP_LOADNIL:
			if b := opGetArgB(inst); a <= reg && reg <= b {
				set = pc
			}
		case OP_TFORLOOP:
			if reg >= a+2 {
				set = pc
			}
		case OP_CALL, OP_TAILCALL:
			if reg >= a {
				set = pc
			}
		case OP_JMP:
			b := opGetArgSbx(inst)
			if dest := pc + 1 + b; pc < dest && dest <= lastpc {
				pc += b
			}
		default:
			if op <= opCodeMax && opProps[op].SetRegA && reg == a {
				set = pc
			}
		}
	}
	return set
}

// constName is the name a constant stands for: a string, or "?".
func (p *FunctionProto) constName(rk int) string {
	if opIsK(rk) {
		if s, ok := p.Constants[opIndexK(rk)].(LString); ok {
			return string(s)
		}
	}
	return "?"
}

// objName says what register reg holds at the instruction at pc: its kind
// ("local", "global", "field", "method", "upvalue", "constant") and name, or
// "" when it cannot tell.
func (p *FunctionProto) objName(reg, pc int) (kind, name string) {
	if name = p.localName(reg, pc); name != "" {
		return "local", name
	}
	set := p.setsReg(pc, reg)
	if set < 0 {
		return "", ""
	}
	inst := p.Code[set]
	switch opGetOpCode(inst) {
	case OP_GETGLOBAL:
		if s, ok := p.Constants[opGetArgBx(inst)].(LString); ok {
			return "global", string(s)
		}
	case OP_MOVE:
		if b := opGetArgB(inst); b < opGetArgA(inst) {
			return p.objName(b, set)
		}
	case OP_GETTABLE, OP_GETTABLEKS:
		return "field", p.constName(opGetArgC(inst))
	case OP_GETUPVAL:
		if b := opGetArgB(inst); b < len(p.DbgUpvalues) {
			return "upvalue", p.DbgUpvalues[b]
		}
	case OP_LOADK:
		if s, ok := p.Constants[opGetArgBx(inst)].(LString); ok {
			return "constant", string(s)
		}
	case OP_SELF:
		return "method", p.constName(opGetArgC(inst))
	}
	return "", ""
}

// currentInst is the instruction a Lua function is running, with its
// prototype and place; ok is false when the state is in a Go function.
func (ls *LState) currentInst() (p *FunctionProto, pc int, inst uint32, ok bool) {
	cf := ls.currentFrame
	if cf == nil || cf.Fn == nil || cf.Fn.IsG || cf.Fn.Proto == nil {
		return nil, 0, 0, false
	}
	p, pc = cf.Fn.Proto, cf.Pc-1
	if pc < 0 || pc >= len(p.Code) {
		return nil, 0, 0, false
	}
	return p, pc, p.Code[pc], true
}

// typeError raises "attempt to <what> <kind> '<name>' (a <type> value)" for
// the value v, which was in register reg of the running function (-1 when
// it was not in one).
func (ls *LState) typeError(what string, v LValue, reg int) {
	if p, pc, _, ok := ls.currentInst(); ok && reg >= 0 {
		if kind, name := p.objName(reg, pc); kind != "" {
			ls.RaiseError("attempt to %s %s '%s' (a %s value)", what, kind, name, v.Type().String())
		}
	}
	ls.RaiseError("attempt to %s a %s value", what, v.Type().String())
}

// registerOf is the register an operand of the running instruction is
// in: arg as the instruction has it (a register, or a constant for the
// operands that may be), or -1.
func registerOf(arg int, rk bool) int {
	if rk && opIsK(arg) {
		return -1
	}
	return arg
}

// indexError is the error for indexing a value that cannot be indexed. first
// says it is the value the instruction indexed, not one a metatable's
// __index led to.
func (ls *LState) indexError(v LValue, first bool) {
	reg := -1
	if _, _, inst, ok := ls.currentInst(); ok && first {
		switch opGetOpCode(inst) {
		case OP_GETTABLE, OP_GETTABLEKS, OP_SELF:
			reg = opGetArgB(inst)
		case OP_SETTABLE, OP_SETTABLEKS:
			reg = opGetArgA(inst)
		}
	}
	ls.typeError("index", v, reg)
}

// callError is the error for calling something that is not a function.
func (ls *LState) callError(v LValue) {
	reg := -1
	if _, _, inst, ok := ls.currentInst(); ok {
		switch opGetOpCode(inst) {
		case OP_CALL, OP_TAILCALL:
			reg = opGetArgA(inst)
		}
	}
	ls.typeError("call", v, reg)
}

// arithError is the error for arithmetic on values that are not numbers: it
// blames the left one unless that is a number (or a string that is one), as
// Lua does.
func (ls *LState) arithError(lhs, rhs LValue) {
	bad, operand := rhs, func(inst uint32) int { return opGetArgC(inst) }
	if _, ok := toNumberValue(lhs); !ok {
		bad, operand = lhs, func(inst uint32) int { return opGetArgB(inst) }
	}
	reg := -1
	if _, _, inst, ok := ls.currentInst(); ok {
		switch opGetOpCode(inst) {
		case OP_ADD, OP_SUB, OP_MUL, OP_DIV, OP_MOD, OP_POW:
			reg = registerOf(operand(inst), true)
		}
	}
	ls.typeError("perform arithmetic on", bad, reg)
}

// unaryError is the error for a unary operation on a value it does not work
// on: arithmetic (the minus) or the length.
func (ls *LState) unaryError(what string, v LValue) {
	reg := -1
	if _, _, inst, ok := ls.currentInst(); ok {
		switch opGetOpCode(inst) {
		case OP_UNM, OP_LEN:
			reg = opGetArgB(inst)
		}
	}
	ls.typeError(what, v, reg)
}

// concatError is the error for joining a value that is not a string or a
// number. lreg and rreg are the registers the two were in, or -1.
func (ls *LState) concatError(lhs, rhs LValue, lreg, rreg int) {
	if LVCanConvToString(lhs) {
		ls.typeError("concatenate", rhs, rreg)
	}
	ls.typeError("concatenate", lhs, lreg)
}

// compareError is the error for ordering values that cannot be.
func (ls *LState) compareError(lhs, rhs LValue) {
	t1, t2 := lhs.Type().String(), rhs.Type().String()
	if t1 == t2 {
		ls.RaiseError("attempt to compare two %s values", t1)
	}
	ls.RaiseError("attempt to compare %s with %s", t1, t2)
}

// toNumberValue is what a number is to arithmetic: a number, or a string
// that reads as one.
func toNumberValue(v LValue) (LNumber, bool) {
	switch x := v.(type) {
	case LNumber:
		return x, true
	case LString:
		if n, err := parseNumber(string(x)); err == nil {
			return n, true
		}
	}
	return 0, false
}

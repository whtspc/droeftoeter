package sandbox

import (
	"fmt"

	"github.com/dop251/goja"
	"github.com/mattn/go-runewidth"
)

const (
	GridW = 64
	GridH = 32
)

type Cell struct {
	Char  string
	Color string
}

type Sandbox struct {
	vm          *goja.Runtime
	grid        [GridW][GridH]*Cell
	onTickFn    goja.Callable
	logFn       func(string)
	currentCode string
}

func New(logFn func(string)) *Sandbox {
	s := &Sandbox{
		logFn: logFn,
	}
	s.initRuntime()
	return s
}

func (s *Sandbox) initRuntime() {
	vm := goja.New()
	s.vm = vm
	s.onTickFn = nil

	for x := 0; x < GridW; x++ {
		for y := 0; y < GridH; y++ {
			s.grid[x][y] = nil
		}
	}

	vm.Set("gridW", GridW)
	vm.Set("gridH", GridH)

	vm.RunString(fmt.Sprintf(`
		var grid = [];
		for (var i = 0; i < %d; i++) {
			grid[i] = [];
			for (var j = 0; j < %d; j++) {
				grid[i][j] = null;
			}
		}
	`, GridW, GridH))

	maxX := GridW - 1
	maxY := GridH - 1

	vm.Set("setCell", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 3 {
			return goja.Undefined()
		}
		x := int(call.Arguments[0].ToFloat())
		y := int(call.Arguments[1].ToFloat())
		if x < 0 { x = 0 }
		if x > maxX { x = maxX }
		if y < 0 { y = 0 }
		if y > maxY { y = maxY }

		if call.Arguments[2] == goja.Null() || call.Arguments[2] == goja.Undefined() {
			s.grid[x][y] = nil
			vm.RunString(fmt.Sprintf("grid[%d][%d] = null;", x, y))
			return goja.Undefined()
		}

		char := call.Arguments[2].String()
		// Replace double-width characters (emoji etc.) with a fallback
		// to prevent grid misalignment.
		if r := []rune(char); len(r) > 0 && runewidth.RuneWidth(r[0]) > 1 {
			char = "?"
		}
		color := "#ffffff"
		if len(call.Arguments) >= 4 && call.Arguments[3] != goja.Undefined() && call.Arguments[3] != goja.Null() {
			color = call.Arguments[3].String()
		}

		s.grid[x][y] = &Cell{Char: char, Color: color}
		vm.RunString(fmt.Sprintf(`grid[%d][%d] = {char: "%s", color: "%s"};`, x, y, char, color))
		return goja.Undefined()
	})

	vm.Set("clearGrid", func(call goja.FunctionCall) goja.Value {
		for x := 0; x < GridW; x++ {
			for y := 0; y < GridH; y++ {
				s.grid[x][y] = nil
			}
		}
		vm.RunString(fmt.Sprintf(`
			for (var i = 0; i < %d; i++) {
				for (var j = 0; j < %d; j++) {
					grid[i][j] = null;
				}
			}
		`, GridW, GridH))
		return goja.Undefined()
	})

	vm.Set("onTick", func(fn goja.Callable) {
		s.onTickFn = fn
	})

	vm.Set("log", func(call goja.FunctionCall) goja.Value {
		msg := ""
		for i, arg := range call.Arguments {
			if i > 0 {
				msg += " "
			}
			msg += arg.String()
		}
		s.logFn(msg)
		return goja.Undefined()
	})
}

func (s *Sandbox) Reset() {
	s.currentCode = ""
	s.initRuntime()
}

func (s *Sandbox) Inject(code string) error {
	_, err := s.vm.RunString(code)
	if err != nil {
		s.logFn(fmt.Sprintf("[error] %v", err))
		return err
	}
	s.currentCode = code
	return nil
}

func (s *Sandbox) RunTick(tickNum int) {
	if s.onTickFn == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			s.logFn(fmt.Sprintf("[error] tick panic: %v", r))
		}
	}()

	_, err := s.onTickFn(goja.Undefined(), s.vm.ToValue(tickNum))
	if err != nil {
		s.logFn(fmt.Sprintf("[error] tick: %v", err))
	}
}

func (s *Sandbox) GetGrid() *[GridW][GridH]*Cell {
	return &s.grid
}

func (s *Sandbox) CurrentCode() string {
	return s.currentCode
}

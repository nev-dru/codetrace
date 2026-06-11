package main

type Greeter interface{ Greet() string }

type English struct{}

func (English) Greet() string { return "hi" }

type French struct{}

func (French) Greet() string { return "salut" } // French never instantiated: VTA should exclude

func pick() Greeter { return English{} }

func helperA() string { return helperB() }
func helperB() string { return "b" }

func unused() {} // dead

func recurseX() { recurseY() }
func recurseY() { recurseX() }

func main() {
	g := pick()
	_ = g.Greet()
	_ = helperA()
	recurseX()
}

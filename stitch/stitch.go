//go:generate ../scripts/generate-bindings bindings.js

package stitch

import (
	"encoding/json"

	"github.com/robertkrimen/otto"

	// Automatically import the Javascript underscore utility-belt library into
	// the Stitch VM.
	_ "github.com/robertkrimen/otto/underscore"

	"github.com/NetSys/quilt/util"
)

// A Stitch is an abstract representation of the policy language.
type Stitch struct {
	Containers  []Container
	Labels      []Label
	Connections []Connection
	Placements  []Placement
	Machines    []Machine

	AdminACL  []string
	MaxPrice  float64
	Namespace string

	Invariants []invariant
}

// A Placement constraint guides where containers may be scheduled, either relative to
// the labels of other containers, or the machine the container will run on.
type Placement struct {
	TargetLabel string

	Exclusive bool

	// Label Constraint
	OtherLabel string

	// Machine Constraints
	Provider string
	Size     string
	Region   string
}

// A Container may be instantiated in the stitch and queried by users.
type Container struct {
	ID      int
	Image   string
	Command []string
	Env     map[string]string
}

// A Label represents a logical group of containers.
type Label struct {
	Name        string
	IDs         []int
	Annotations []string
}

// A Connection allows containers implementing the From label to speak to containers
// implementing the To label in ports in the range [MinPort, MaxPort]
type Connection struct {
	From    string
	To      string
	MinPort int
	MaxPort int
}

// A ConnectionSlice allows for slices of Collections to be used in joins
type ConnectionSlice []Connection

// A Machine specifies the type of VM that should be booted.
type Machine struct {
	Provider string
	Role     string
	Size     string
	CPU      Range
	RAM      Range
	DiskSize int
	Region   string
	SSHKeys  []string
}

// A Range defines a range of acceptable values for a Machine attribute
type Range struct {
	Min float64
	Max float64
}

// PublicInternetLabel is a magic label that allows connections to or from the public
// network.
const PublicInternetLabel = "public"

// Accepts returns true if `x` is within the range specified by `stitchr` (include),
// or if no max is specified and `x` is larger than `stitchr.min`.
func (stitchr Range) Accepts(x float64) bool {
	return stitchr.Min <= x && (stitchr.Max == 0 || x <= stitchr.Max)
}

func run(vm *otto.Otto, filename string, code string) (otto.Value, error) {
	// Compile before running so that stacktraces have filenames.
	script, err := vm.Compile(filename, code)
	if err != nil {
		return otto.Value{}, err
	}

	return vm.Run(script)
}

func newVM(getter ImportGetter) (*otto.Otto, error) {
	vm := otto.New()
	if err := vm.Set("githubKeys", toOttoFunc(githubKeysImpl)); err != nil {
		return vm, err
	}
	if err := vm.Set("require", toOttoFunc(getter.requireImpl)); err != nil {
		return vm, err
	}

	_, err := run(vm, "<javascript_bindings>", javascriptBindings)
	return vm, err
}

// `runSpec` evaluates `spec` within a module closure.
func runSpec(vm *otto.Otto, filename string, spec string) (otto.Value, error) {
	// The function declaration must be prepended to the first line of the
	// import or else stacktraces will show an offset line number.
	exec := "(function() {" +
		"var module={exports: {}};" +
		"(function(module, exports) {" +
		spec +
		"})(module, module.exports);" +
		"return module.exports" +
		"})()"
	return run(vm, filename, exec)
}

// New parses and executes a stitch (in text form), and returns an abstract Dsl handle.
func New(filename string, specStr string, getter ImportGetter) (Stitch, error) {
	vm, err := newVM(getter)
	if err != nil {
		return Stitch{}, err
	}

	if _, err := runSpec(vm, filename, specStr); err != nil {
		return Stitch{}, err
	}

	spec, err := parseContext(vm)
	if err != nil {
		return Stitch{}, err
	}
	spec.createPortRules()

	if len(spec.Invariants) == 0 {
		return spec, nil
	}

	graph, err := InitializeGraph(spec)
	if err != nil {
		return Stitch{}, err
	}

	if err := checkInvariants(graph, spec.Invariants); err != nil {
		return Stitch{}, err
	}

	return spec, nil
}

// FromJavascript gets a Stitch handle from a string containing Javascript code.
func FromJavascript(specStr string, getter ImportGetter) (Stitch, error) {
	return New("<raw_string>", specStr, getter)
}

// FromFile gets a Stitch handle from a file on disk.
func FromFile(filename string, getter ImportGetter) (Stitch, error) {
	specStr, err := util.ReadFile(filename)
	if err != nil {
		return Stitch{}, err
	}
	return New(filename, specStr, getter)
}

// FromJSON gets a Stitch handle from the deployment representation.
func FromJSON(jsonStr string) (stc Stitch, err error) {
	err = json.Unmarshal([]byte(jsonStr), &stc)
	return stc, err
}

func parseContext(vm *otto.Otto) (stc Stitch, err error) {
	vmCtx, err := vm.Run("deployment.toQuiltRepresentation()")
	if err != nil {
		return stc, err
	}

	// Export() always returns `nil` as the error (it's only present for
	// backwards compatibility), so we can safely ignore it.
	exp, _ := vmCtx.Export()
	ctxStr, err := json.Marshal(exp)
	if err != nil {
		return stc, err
	}
	err = json.Unmarshal(ctxStr, &stc)
	return stc, err
}

// createPortRules creates exclusive placement rules such that no two containers
// listening on the same public port get placed on the same machine.
func (stitch *Stitch) createPortRules() {
	ports := make(map[int][]string)
	for _, c := range stitch.Connections {
		if c.From != PublicInternetLabel && c.To != PublicInternetLabel {
			continue
		}

		target := c.From
		if c.From == PublicInternetLabel {
			target = c.To
		}

		min := c.MinPort
		ports[min] = append(ports[min], target)
	}

	for _, labels := range ports {
		for _, tgt := range labels {
			for _, other := range labels {
				stitch.Placements = append(stitch.Placements,
					Placement{
						Exclusive:   true,
						TargetLabel: tgt,
						OtherLabel:  other,
					})
			}
		}
	}
}

// String returns the Stitch in its deployment representation.
func (stitch Stitch) String() string {
	jsonBytes, err := json.Marshal(stitch)
	if err != nil {
		panic(err)
	}
	return string(jsonBytes)
}

// Get returns the value contained at the given index
func (cs ConnectionSlice) Get(ii int) interface{} {
	return cs[ii]
}

// Len returns the number of items in the slice
func (cs ConnectionSlice) Len() int {
	return len(cs)
}

func stitchError(vm *otto.Otto, err error) otto.Value {
	return vm.MakeCustomError("StitchError", err.Error())
}

// toOttoFunc converts functions that return an error as a return value into
// a function that panics on errors. Otto requires functions to panic to signify
// errors in order to generate a stack trace.
func toOttoFunc(fn func(otto.FunctionCall) (otto.Value, error)) func(
	otto.FunctionCall) otto.Value {

	return func(call otto.FunctionCall) otto.Value {
		res, err := fn(call)
		if err != nil {
			// Otto uses `panic` with `*otto.Error`s to signify Javascript
			// runtime errors.
			if _, ok := err.(*otto.Error); ok {
				panic(err)
			}
			panic(stitchError(call.Otto, err))
		}
		return res
	}
}

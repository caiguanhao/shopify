package shopify

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

var (
	reOperationArgs = regexp.MustCompile(`([\w!]+)\s*:\s*([\w!]+)`)
)

type (
	Multi struct {
		operationType string
		items         []*MultiItem
	}

	MultiItem struct {
		multi      *Multi
		operation  string
		opArgTypes [][]string
		bodyType   string
		body       string
		inputs     []interface{}
		targets    []interface{}
	}
)

// NewMulti creates a chain to define operations and output destinations,
// allowing to generate one single GraphQL query to execute multiple GraphQL
// queries or mutations at the same time.
func NewMulti(operationType string) *Multi {
	return &Multi{
		operationType: operationType,
	}
}

// Add defines a new operation with its argument types. The operation argument
// types must be in the format of "name: type".
func (m *Multi) Add(operation string, operationArgTypes ...string) *MultiItem {
	argTypes := reOperationArgs.FindAllStringSubmatch(strings.Join(operationArgTypes, ", "), -1)
	mi := &MultiItem{
		multi:      m,
		operation:  operation,
		opArgTypes: argTypes,
	}
	m.items = append(m.items, mi)
	return mi

}

// Do generates gql, args and targets that can be used in Client.New() and
// Request.Do().
func (m *Multi) Do() (gql string, args, targets []interface{}) {
	var gqlIns []string
	var ops []string
	var fragments []string
	c, d, f := 0, 0, 0
	for _, item := range m.items {
		x := 0
		body := item.body
		if item.bodyType != "" {
			fragments = append(fragments, fmt.Sprintf("fragment frag%d on %s %s\n", f, item.bodyType, body))
			body = fmt.Sprintf("{ ...frag%d }", f)
			f += 1
		}
		var numberOfItems int
		if len(item.opArgTypes) > 0 {
			numberOfItems = len(item.inputs) / len(item.opArgTypes)
		}
		if numberOfItems < 1 {
			numberOfItems = 1
		}
		for i := 0; i < numberOfItems; i++ {
			var opIns []string
			for j := range item.opArgTypes {
				name := item.opArgTypes[j][1]
				typ := item.opArgTypes[j][2]
				opIns = append(opIns, fmt.Sprintf("%s: $%s%d", name, name, c))
				gqlIns = append(gqlIns, fmt.Sprintf("$%s%d: %s", name, c, typ))
				args = append(args, fmt.Sprintf("%s%d", name, c), item.inputs[x])
				x += 1
			}
			opIn := strings.Join(opIns, ", ")
			if opIn != "" {
				opIn = "(" + opIn + ")"
			}
			ops = append(ops, fmt.Sprintf("gql%d: %s%s %s\n", c, item.operation, opIn, body))
			c += 1
		}
		for n := 0; n < len(item.targets)/2; n++ {
			path, ok := item.targets[2*n+1].(string)
			if !ok {
				continue
			}
			rv := reflect.Indirect(reflect.ValueOf(item.targets[2*n]))
			if rv.Kind() == reflect.Slice {
				if elemType := rv.Type().Elem(); elemType.Kind() == reflect.Slice { // [][]type
					rv.Set(reflect.MakeSlice(reflect.SliceOf(elemType), numberOfItems, numberOfItems))
					for i := 0; i < numberOfItems; i++ {
						targets = append(targets, rv.Index(i).Addr().Interface(), fmt.Sprintf("gql%d%s", d+i, path))
					}
					continue
				}
			}
			targets = append(targets, item.targets[2*n], fmt.Sprintf("gql%d%s", d+numberOfItems-1, path))
			continue
		}
		d += numberOfItems
	}
	gqlIn := strings.Join(gqlIns, ", ")
	if gqlIn != "" {
		gqlIn = "(" + gqlIn + ")"
	}
	gql = strings.Join(fragments, "") + m.operationType + gqlIn + " {\n" + strings.Join(ops, "") + "}"
	return
}

// Len returns number of items in the chain.
func (m Multi) Len() int {
	return len(m.items)
}

func (m Multi) String() string {
	var list []string
	for i := range m.items {
		list = append(list, m.items[i].String())
	}
	return strings.Join(list, "\n")
}

// Return defines output fields of a operation. This is the body of the GraphQL
// query. If the body starts with corresponding type, then fragment will be
// used.
func (mi *MultiItem) Return(body string) *MultiItem {
	var bodyType string
	if i := strings.Index(body, "{"); i > -1 {
		bodyType = strings.TrimSpace(body[0:i])
		body = body[i:]
	}
	mi.bodyType = bodyType
	mi.body = body
	return mi
}

// In puts input values in the order of the operation argument types to the chain.
func (mi *MultiItem) In(args ...interface{}) *MultiItem {
	mi.inputs = args
	return mi
}

// Out puts destinations to slices followed by a JSON path to the chain.
func (mi *MultiItem) Out(targets ...interface{}) *MultiItem {
	mi.targets = targets
	return mi
}

// Self returns the chain.
func (mi *MultiItem) Self() *Multi {
	return mi.multi
}

func (mi MultiItem) String() string {
	var list []string
	for i := range mi.opArgTypes {
		list = append(list, mi.opArgTypes[i][1]+": "+mi.opArgTypes[i][2])
	}
	args := strings.Join(list, ", ")
	if args != "" {
		args = "(" + args + ")"
	}
	return mi.operation + args + " " + mi.body
}

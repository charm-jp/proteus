package rpc

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/charm-jp/proteus/protobuf"
	"github.com/charm-jp/proteus/resolver"
	"github.com/charm-jp/proteus/scanner"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RPCSuite struct {
	suite.Suite
	g *Generator
}

func (s *RPCSuite) SetupTest() {
	s.g = NewGenerator()
}

const expectedImplType = "type Foo struct {\n}"

func (s *RPCSuite) TestDeclImplType() {
	output, err := render(s.g.declImplType("Foo"))
	s.Nil(err)
	s.Equal(expectedImplType, output)
}

const expectedConstructor = `func NewFoo() *Foo {
	return &Foo{}
}`

func (s *RPCSuite) TestDeclConstructor() {
	output, err := render(s.g.declConstructor("Foo", "NewFoo"))
	s.Nil(err)
	s.Equal(expectedConstructor, output)
}

const expectedFuncNotGenerated = `func (s *FooServer) DoFoo(ctx xcontext.Context, in *Foo) (result *Bar, err error) {
	result = new(Bar)
	result = DoFoo(in)
	return
}`

const expectedFuncNotGeneratedCtx = `func (s *FooServer) DoFooCtx(ctx xcontext.Context, in *Foo) (result *Bar, err error) {
	result = new(Bar)
	result = DoFooCtx(ctx, in)
	return
}`

const expectedFuncNotGeneratedAndNotNullable = `func (s *FooServer) DoFoo(ctx xcontext.Context, in *Foo) (result *Bar, err error) {
	result = new(Bar)
	aux := DoFoo(in)
	result = &aux
	return
}`

const expectedFuncNotGeneratedAndNotNullableIn = `func (s *FooServer) DoFoo(ctx xcontext.Context, in *Foo) (result *Bar, err error) {
	result = new(Bar)
	result = DoFoo(*in)
	return
}`

const expectedFuncGenerated = `func (s *FooServer) DoFoo(ctx xcontext.Context, in *FooRequest) (result *FooResponse, err error) {
	result = new(FooResponse)
	result.Result1, result.Result2, result.Result3 = DoFoo(in.Arg1, in.Arg2, in.Arg3)
	return
}`

const expectedFuncGeneratedVariadic = `func (s *FooServer) DoFoo(ctx xcontext.Context, in *FooRequest) (result *FooResponse, err error) {
	result = new(FooResponse)
	result.Result1, result.Result2, result.Result3 = DoFoo(in.Arg1, in.Arg2, in.Arg3...)
	return
}`

const expectedFuncGeneratedWithError = `func (s *FooServer) DoFoo(ctx xcontext.Context, in *FooRequest) (result *FooResponse, err error) {
	result = new(FooResponse)
	result.Result1, result.Result2, result.Result3, err = DoFoo(in.Arg1, in.Arg2, in.Arg3)
	return
}`

const expectedMethod = `func (s *FooServer) Fooer_DoFoo(ctx xcontext.Context, in *FooRequest) (result *FooResponse, err error) {
	result = new(FooResponse)
	result.Result1, result.Result2, result.Result3, err = s.Fooer.DoFoo(in.Arg1, in.Arg2, in.Arg3)
	return
}`

const expectedMethodExternalInput = `func (s *FooServer) T_Foo(ctx xcontext.Context, in *ast.BlockStmt) (result *T_FooResponse, err error) {
	result = new(T_FooResponse)
	_ = s.T.Foo(in)
	return
}`

const expectedFuncEmptyInAndOut = `func (s *FooServer) Empty(ctx xcontext.Context, in *Empty) (result *Empty, err error) {
	Empty()
	return
}`

const expectedFuncEmptyInAndOutWithError = `func (s *FooServer) Empty(ctx xcontext.Context, in *Empty) (result *Empty, err error) {
	err = Empty()
	return
}`

func (s *RPCSuite) TestDeclMethod() {
	cases := []struct {
		name   string
		rpc    *protobuf.RPC
		output string
	}{
		{
			"func not generated",
			&protobuf.RPC{
				Name:   "DoFoo",
				Method: "DoFoo",
				Input:  nullable(protobuf.NewNamed("", "Foo")),
				Output: nullable(protobuf.NewNamed("", "Bar")),
			},
			expectedFuncNotGenerated,
		},
		{
			"func not generated with ctx",
			&protobuf.RPC{
				Name:   "DoFooCtx",
				Method: "DoFooCtx",
				HasCtx: true,
				Input:  nullable(protobuf.NewNamed("", "Foo")),
				Output: nullable(protobuf.NewNamed("", "Bar")),
			},
			expectedFuncNotGeneratedCtx,
		},
		{
			"func output not generated and not nullable",
			&protobuf.RPC{
				Name:   "DoFoo",
				Method: "DoFoo",
				Input:  nullable(protobuf.NewNamed("", "Foo")),
				Output: notNullable(protobuf.NewNamed("", "Bar")),
			},
			expectedFuncNotGeneratedAndNotNullable,
		},
		{
			"func output not generated and not nullable input",
			&protobuf.RPC{
				Name:   "DoFoo",
				Method: "DoFoo",
				Input:  notNullable(protobuf.NewNamed("", "Foo")),
				Output: nullable(protobuf.NewNamed("", "Bar")),
			},
			expectedFuncNotGeneratedAndNotNullableIn,
		},
		{
			"func generated",
			&protobuf.RPC{
				Name:   "DoFoo",
				Method: "DoFoo",
				Input:  nullable(protobuf.NewGeneratedNamed("", "FooRequest")),
				Output: nullable(protobuf.NewGeneratedNamed("", "FooResponse")),
			},
			expectedFuncGenerated,
		},
		{
			"func generated with variadic arg",
			&protobuf.RPC{
				Name:       "DoFoo",
				Method:     "DoFoo",
				Input:      nullable(protobuf.NewGeneratedNamed("", "FooRequest")),
				Output:     nullable(protobuf.NewGeneratedNamed("", "FooResponse")),
				IsVariadic: true,
			},
			expectedFuncGeneratedVariadic,
		},
		{
			"func generated with error",
			&protobuf.RPC{
				Name:     "DoFoo",
				Method:   "DoFoo",
				HasError: true,
				Input:    nullable(protobuf.NewGeneratedNamed("", "FooRequest")),
				Output:   nullable(protobuf.NewGeneratedNamed("", "FooResponse")),
			},
			expectedFuncGeneratedWithError,
		},
		{
			"method call",
			&protobuf.RPC{
				Name:     "Fooer_DoFoo",
				Method:   "DoFoo",
				Recv:     "Fooer",
				HasError: true,
				Input:    nullable(protobuf.NewGeneratedNamed("", "FooRequest")),
				Output:   nullable(protobuf.NewGeneratedNamed("", "FooResponse")),
			},
			expectedMethod,
		},
		{
			"method with external type input",
			&protobuf.RPC{
				Name:     "T_Foo",
				Method:   "Foo",
				Recv:     "T",
				HasError: false,
				Input:    nullable(protobuf.NewNamed("go.ast", "BlockStmt")),
				Output:   nullable(protobuf.NewGeneratedNamed("", "T_FooResponse")),
			},
			expectedMethodExternalInput,
		},
		{
			"func with empty input and output",
			&protobuf.RPC{
				Name:   "Empty",
				Method: "Empty",
				Input:  nullable(protobuf.NewGeneratedNamed("", "Empty")),
				Output: nullable(protobuf.NewGeneratedNamed("", "Empty")),
			},
			expectedFuncEmptyInAndOut,
		},
		{
			"func with empty input and output with error",
			&protobuf.RPC{
				Name:     "Empty",
				Method:   "Empty",
				HasError: true,
				Input:    nullable(protobuf.NewGeneratedNamed("", "Empty")),
				Output:   nullable(protobuf.NewGeneratedNamed("", "Empty")),
			},
			expectedFuncEmptyInAndOutWithError,
		},
	}

	proto := &protobuf.Package{
		Messages: []*protobuf.Message{
			&protobuf.Message{
				Name: "FooRequest",
				Fields: []*protobuf.Field{
					&protobuf.Field{
						Name:     "FirstField",
						Pos:      1,
						Repeated: false,
						Type:     protobuf.NewBasic("int64"),
					},
					&protobuf.Field{
						Name:     "SecondField",
						Pos:      2,
						Repeated: false,
						Type:     protobuf.NewBasic("string"),
					},
					&protobuf.Field{
						Name:     "ThirdField",
						Pos:      3,
						Repeated: false,
						Type:     protobuf.NewBasic("string"),
					},
				},
			},
			&protobuf.Message{
				Name: "FooResponse",
				Fields: []*protobuf.Field{
					&protobuf.Field{
						Name:     "PrimerField",
						Pos:      1,
						Repeated: false,
						Type:     protobuf.NewBasic("int64"),
					},
					&protobuf.Field{
						Name:     "SegundoField",
						Pos:      2,
						Repeated: false,
						Type:     protobuf.NewBasic("string"),
					},
					&protobuf.Field{
						Name:     "TercerField",
						Pos:      3,
						Repeated: false,
						Type:     protobuf.NewBasic("string"),
					},
				},
			},
			&protobuf.Message{
				Name:   "T_FooResponse",
				Fields: make([]*protobuf.Field, 1),
			},
			&protobuf.Message{
				Name: "Empty",
			},
		},
	}

	ctx := &context{
		implName: "FooServer",
		proto:    proto,
		pkg:      s.fakePkg(),
	}

	for _, c := range cases {
		output, err := render(s.g.declMethod(ctx, c.rpc))
		s.Nil(err, c.name, c.name)
		s.Equal(c.output, output, c.name)
	}
}

const expectedGeneratedFile = `package subpkg

import (
	xcontext "golang.org/x/net/context"
)

type subpkgServiceServer struct {
}

func NewSubpkgServiceServer() *subpkgServiceServer {
	return &subpkgServiceServer{}
}
func (s *subpkgServiceServer) Generated(ctx xcontext.Context, in *GeneratedRequest) (result *GeneratedResponse, err error) {
	result = new(GeneratedResponse)
	result.Result1, err = Generated(in.Arg1)
	return
}
func (s *subpkgServiceServer) MyContainer_Name(ctx xcontext.Context, in *MyContainer_NameRequest) (result *MyContainer_NameResponse, err error) {
	result = new(MyContainer_NameResponse)
	result.Result1 = s.MyContainer.Name()
	return
}
func (s *subpkgServiceServer) Point_GeneratedMethod(ctx xcontext.Context, in *Point_GeneratedMethodRequest) (result *Point, err error) {
	result = new(Point)
	result = s.Point.GeneratedMethod(in.Arg1)
	return
}
func (s *subpkgServiceServer) Point_GeneratedMethodOnPointer(ctx xcontext.Context, in *Point_GeneratedMethodOnPointerRequest) (result *Point, err error) {
	result = new(Point)
	result = s.Point.GeneratedMethodOnPointer(in.Arg1)
	return
}
`

func (s *RPCSuite) TestGenerate() {
	pkg := "github.com/charm-jp/proteus/fixtures/subpkg"
	scanner, err := scanner.New(pkg)
	s.Nil(err)

	pkgs, err := scanner.Scan()
	s.Nil(err)

	r := resolver.New()
	r.Resolve(pkgs)

	t := protobuf.NewTransformer()
	s.Nil(s.g.Generate(t.Transform(pkgs[0]), pkg))

	data, err := ioutil.ReadFile(projectPath("fixtures/subpkg/server.proteus.go"))
	s.Nil(err)
	s.Equal(expectedGeneratedFile, string(data))

	s.Nil(os.Remove(projectPath("fixtures/subpkg/server.proteus.go")))
}

func TestServiceImplName(t *testing.T) {
	require.Equal(t, "fooServiceServer", serviceImplName(&protobuf.Package{
		Name: "foo",
	}))
}

func TestConstructorName(t *testing.T) {
	require.Equal(t, "NewFooServiceServer", constructorName(&protobuf.Package{
		Name: "foo",
	}))
}

const testPkg = `package fake

import "go/ast"
import "context"

type Foo struct{}
type Bar struct {}

func DoFoo(in *Foo) *Bar {
	return nil
}

func DoFooCtx(ctx context.Context, in *Foo) *Bar {
	return nil
}

func MoreFoo(a int) *ast.BlockStmt {
	return nil
}

type T struct{}

func (*T) Foo(s *ast.BlockStmt) int {
	return 0
}
`

func (s *RPCSuite) fakePkg() *types.Package {
	fs := token.NewFileSet()

	f, err := parser.ParseFile(fs, "src.go", testPkg, 0)
	if err != nil {
		panic(err)
	}

	config := types.Config{
		FakeImportC: true,
		Importer:    importer.Default(),
	}

	pkg, err := config.Check("", fs, []*ast.File{f}, nil)
	s.Nil(err)
	return pkg
}

func nullable(t protobuf.Type) protobuf.Type {
	src := scanner.NewNamed("", "X")
	src.SetNullable(true)
	t.SetSource(src)
	return t
}

func notNullable(t protobuf.Type) protobuf.Type {
	src := scanner.NewNamed("", "X")
	src.SetNullable(false)
	t.SetSource(src)
	return t
}

func render(decl ast.Decl) (string, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), decl); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func TestRPCSuite(t *testing.T) {
	suite.Run(t, new(RPCSuite))
}

func projectPath(path string) string {
	return filepath.Join(os.Getenv("GOPATH"), "src", "github.com/charm-jp/proteus", path)
}

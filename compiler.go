package fhirpath

import (
	"errors"
	"fmt"

	"github.com/antlr4-go/antlr/v4"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/parser/grammar"
)

// errorListener captures parsing errors.
type errorListener struct {
	*antlr.DefaultErrorListener
	errors []string
}

func (l *errorListener) SyntaxError(_ antlr.Recognizer, _ interface{}, line, column int, msg string, _ antlr.RecognitionException) {
	l.errors = append(l.errors, fmt.Sprintf("line %d:%d %s", line, column, msg))
}

// compile parses a FHIRPath expression into a compiled Expression.
func compile(expr string) (*Expression, error) {
	if expr == "" {
		return nil, fmt.Errorf("empty expression")
	}

	// SLL prediction first, which reads no further context than the
	// decision at hand and builds far less while predicting. Where only full
	// context would decide, it reports a syntax error, and the parser's errors
	// send the expression to full LL prediction: an expression that parses
	// has the tree LL gives it, and one that does not has LL's errors. A
	// lexer's errors do not depend on the mode
	tree, err := parse(expr, antlr.PredictionModeSLL)
	var lexing *lexerError
	if err != nil && !errors.As(err, &lexing) {
		tree, err = parse(expr, antlr.PredictionModeLL)
	}
	if err != nil {
		return nil, err
	}

	return &Expression{
		source: expr,
		tree:   tree,
		node:   eval.Compile(tree),
	}, nil
}

// lexerError is the lexer's errors on an expression.
type lexerError struct {
	errors []string
}

func (e *lexerError) Error() string {
	return fmt.Sprintf("lexer errors: %v", e.errors)
}

// parse parses an expression with a prediction mode, reporting the lexer's
// errors, or else the parser's.
func parse(expr string, mode int) (*grammar.EntireExpressionContext, error) {
	// Create lexer
	input := antlr.NewInputStream(expr)
	lexer := grammar.NewfhirpathLexer(input)

	// Set up error listener for lexer
	lexerErrors := &errorListener{}
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(lexerErrors)

	// Create parser
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	parser := grammar.NewfhirpathParser(stream)
	parser.GetInterpreter().SetPredictionMode(mode)

	// Set up error listener for parser
	parserErrors := &errorListener{}
	parser.RemoveErrorListeners()
	parser.AddErrorListener(parserErrors)

	// Parse the expression
	tree := parser.EntireExpression()

	// Check for errors
	if len(lexerErrors.errors) > 0 {
		return nil, &lexerError{lexerErrors.errors}
	}
	if len(parserErrors.errors) > 0 {
		return nil, fmt.Errorf("parser errors: %v", parserErrors.errors)
	}

	return tree.(*grammar.EntireExpressionContext), nil
}

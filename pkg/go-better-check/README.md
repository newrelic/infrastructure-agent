# Better Check!

*A Library of Checker Extensions for the Check Package*

This library provides useful extensions to the unit testing framework provided
by the [check.v1](http://labix.org/gocheck) package. It is completely
compatible with `go test`.

The `check` package provides a basic `Checker` interface that is the basis for
assertion predicates. The default package provides basic predicates like
`Equals`, `DeepEquals`, `IsNil`, `HasLen`, etc.

The goal of this library is to provide additional predicates that may be
commonly needed for your tests.

## Usage

In your \*\_test.go files where you want to use this. Then begin to use the
predicates in your tests. Simple!

## Testing and Contributing

This package comes with its own basic test suite. When adding new Checkers, be
sure to also add some tests. Checkers should also come with inline
documentation.


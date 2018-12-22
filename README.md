wikiassignment
========

[![GoDoc Reference](https://godoc.org/github.com/ebonetti/wikiassignment?status.svg)](http://godoc.org/github.com/ebonetti/wikiassignment)
[![Build Status](https://travis-ci.org/ebonetti/wikiassignment.svg?branch=master)](https://travis-ci.org/ebonetti/wikiassignment)
[![Coverage Status](https://coveralls.io/repos/ebonetti/wikiassignment/badge.svg?branch=master)](https://coveralls.io/r/ebonetti/wikiassignment?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/ebonetti/wikiassignment)](https://goreportcard.com/report/github.com/ebonetti/wikiassignment)

Description
-----------

Package wikiassignment is a golang package that provides utility functions for automatically assigning wikipedia pages to topics.

Installation
------------

This package can be installed with the go get command:

    go get github.com/ebonetti/wikiassignment/...

Dependencies
-------------

This package depends on `PETSc`. The associated dockerfile provides a complete environment in which use this package. Otherwise `PETSc` can be installed following the same steps as in the dockerfile or in [the PETSc installation page](https://www.mcs.anl.gov/petsc/documentation/installation.html).

Documentation
-------------
API documentation can be found in the [associated godoc reference](https://godoc.org/github.com/ebonetti/wikiassignment)
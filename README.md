wikiassignment
========

[![GoDoc Reference](https://godoc.org/github.com/ebonetti/wikiassignment?status.svg)](http://godoc.org/github.com/ebonetti/wikiassignment)
[![Build Status](https://travis-ci.org/ebonetti/wikiassignment.svg?branch=master)](https://travis-ci.org/ebonetti/wikiassignment)
[![Coverage Status](https://coveralls.io/repos/ebonetti/wikiassignment/badge.svg?branch=master)](https://coveralls.io/r/ebonetti/wikiassignment?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/ebonetti/wikiassignment)](https://goreportcard.com/report/github.com/ebonetti/wikiassignment)

Description
-----------

Package wikiassignment is a golang package that provides utility functions for automatically assigning wikipedia pages to topics. 

Documentation
-------------
API documentation can be found in the [associated godoc reference](https://godoc.org/github.com/ebonetti/wikiassignment)

Installation
------------

This package can be installed with the go get command:

    go get github.com/ebonetti/wikiassignment/...

Requirements
-------------

You will need a machine with internet connection, 16GB of RAM (for the english version) and [docker storage base directory properly setted](https://forums.docker.com/t/how-do-i-change-the-docker-image-installation-directory/1169).

This package depends on `PETSc`. The associated dockerfile provides a complete environment in which use this package. Otherwise `PETSc` can be installed following the same steps as in the dockerfile or in [the PETSc installation page](https://www.mcs.anl.gov/petsc/documentation/installation.html).

Examples of use
-------------

1. `docker run ebonetti/wikibrief export -lang en`: basic usage, run the image on the english nationalization and store the result in the in-containter `/data` folder.
2. `docker run -v /path/2/out/dir:/data ebonetti/wikibrief -d export -lang en`:
..1. run the image as before.
..2. [mount as a volume](https://docs.docker.com/storage/volumes/) the guest `/data` folder to the host folder `/path/2/out/dir`, the output folder, so that at the end of the operations  `/path/2/out/dir` will contain the result. This folder can be changed to an arbitrary folder of your choice.
..3. run the image in detatched mode.
For further explanations please refer to [docker run reference](https://docs.docker.com/engine/reference/run)

Useful commands
-------------
1. `docker pull ebonetti/wikibrief` Update the image to the last revision.
2. `docker kill --signal=SIGQUIT  $(docker ps -ql)` Quit the last container and log trace dump.
4. `docker logs -f $(docker ps -lq)` Fetch the logs of the last container.
5. `docker system prune -fa --volumes` Remove all unused images and volume without asking for confirmation.
wikiassignment
========

[![GoDoc Reference](https://godoc.org/github.com/negapedia/wikiassignment?status.svg)](https://godoc.org/github.com/negapedia/wikiassignment)
[![Build Status](https://travis-ci.org/negapedia/wikiassignment.svg?branch=develop)](https://travis-ci.org/negapedia/wikiassignment)
[![Go Report Card](https://goreportcard.com/badge/github.com/negapedia/wikiassignment)](https://goreportcard.com/report/github.com/negapedia/wikiassignment)
[![Bugs](https://sonarcloud.io/api/project_badges/measure?project=negapedia_wikiassignment&metric=bugs)](https://sonarcloud.io/dashboard?id=negapedia_wikiassignment)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=negapedia_wikiassignment&metric=coverage)](https://sonarcloud.io/dashboard?id=negapedia_wikiassignment)
[![Lines of Code](https://sonarcloud.io/api/project_badges/measure?project=negapedia_wikiassignment&metric=ncloc)](https://sonarcloud.io/dashboard?id=negapedia_wikiassignment)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=negapedia_wikiassignment&metric=sqale_rating)](https://sonarcloud.io/dashboard?id=negapedia_wikiassignment)
[![Reliability Rating](https://sonarcloud.io/api/project_badges/measure?project=negapedia_wikiassignment&metric=reliability_rating)](https://sonarcloud.io/dashboard?id=negapedia_wikiassignment)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=negapedia_wikiassignment&metric=security_rating)](https://sonarcloud.io/dashboard?id=negapedia_wikiassignment)
[![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=negapedia_wikiassignment&metric=vulnerabilities)](https://sonarcloud.io/dashboard?id=negapedia_wikiassignment)
Description
-----------

Package wikiassignment is a golang package that provides utility functions for automatically assigning wikipedia pages to topics. 

Documentation
-------------
API documentation can be found in the [associated godoc reference](https://godoc.org/github.com/negapedia/wikiassignment).

Topics data can be found in [overpedia](https://github.com/negapedia/overpedia/tree/master/nationalization/languages).

Installation
------------

This package can be installed with the go get command:

    go get github.com/negapedia/wikiassignment/...

Requirements
-------------

You will need a machine with internet connection, 16GB of RAM (for the english version) and [docker storage base directory properly setted](https://forums.docker.com/t/how-do-i-change-the-docker-image-installation-directory/1169).

This package depends on `PETSc`. The associated dockerfile provides a complete environment in which use this package. Otherwise `PETSc` can be installed following the same steps as in the dockerfile or in [the PETSc installation page](https://www.mcs.anl.gov/petsc/documentation/installation.html).

Export options
-------------
1. `lang`: [wikipedia nationalization to parse](https://github.com/negapedia/overpedia/tree/master/nationalization/internal/languages) or custom JSON, default `it`.
2. `date`: wikipedia dump date in the format AAAAMMDD, default `latest`.

Examples of use
-------------

1. `docker run negapedia/wikiassignment export -lang en -date 20060102`: basic usage, run the image on the english nationalization dump in date 2 January 2006 and store the result in the in-containter `/data` folder, containing:
..1. `semanticgraph.json` maps source page ID to the array of target page IDs.
..2. `partition.json` maps typology of node (article,category or topic) to the array of page IDs belonging to it.
..3. `absorptionprobabilities.csv` represents each page in a row with its ID and the weight assignment for each topic.
..4. `pages.csv` represents pages in the form requested by wiki2overpediadb.
2. `docker run -v /path/2/out/dir:/data negapedia/wikiassignment -d export -lang en`:
..1. run the image as before.
..2. [mount as a volume](https://docs.docker.com/storage/volumes/) the guest `/data` folder to the host folder `/path/2/out/dir`, the output folder, so that at the end of the operations  `/path/2/out/dir` will contain the result. This folder can be changed to an arbitrary folder of your choice.
..3. run the image in detatched mode.
For further explanations please refer to [docker run reference](https://docs.docker.com/engine/reference/run).

Useful commands
-------------
1. `docker pull negapedia/wikiassignment` Update the image to the last revision.
2. `docker kill --signal=SIGQUIT  $(docker ps -ql)` Quit the last container and log trace dump.
4. `docker logs -f $(docker ps -ql)` Fetch the logs of the last container.
5. `docker system prune -fa --volumes` Remove all unused images and volume without asking for confirmation.
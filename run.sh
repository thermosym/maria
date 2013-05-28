#!/bin/bash

go build -v -o test . && ./test $*


#!/bin/sh

gosec ./...

osv-scanner -r .

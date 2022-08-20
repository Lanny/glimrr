#!/usr/bin/env bash

go run ./src &
last_pid=$!

while read line
do
  kill -s INT $last_pid
  clear
  go run ./src &
  last_pid=$!
done


#!/usr/bin/env bash

go build -o glimrr ./src
./glimrr &
last_pid=$!

while read line
do
  while kill -s INT $last_pid; do 
      sleep 0.1
  done
  clear
  go build -o glimrr ./src
  ./glimrr &
  last_pid=$!
done


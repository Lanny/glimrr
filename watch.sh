#!/usr/bin/env bash

go build -o glimrr ./src
if [[ $? -eq 0 ]]; then
  ./glimrr &
  last_pid=$!
fi

while read line
do
  while kill -s INT $last_pid; do 
      sleep 0.1
  done
  clear
  go build -o glimrr ./src
  if [[ $? -eq 0 ]]; then
    ./glimrr &
    last_pid=$!
  fi
done


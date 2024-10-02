#!/usr/bin/env bash
for i in $OUTPUT_PATH/*; do
   $VECTOR test $i
   if [ "$?" -ne 0 ]; then
      exit 1
   fi
done

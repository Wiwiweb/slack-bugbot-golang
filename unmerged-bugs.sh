#!/bin/bash
cd stackato-release
git checkout master --quiet
git pull --quiet
git submodule update --init --recursive --quiet
git branch -r --no-merged | grep "[0-9]\{6\}" | while read branch; do echo "$(git log -1 --format=%an $branch)" "$branch"; done | sort | grep -v Entering | sed -e 's/.*\([0-9]\{6\}\).*/\1/' | egrep '^3'
git submodule foreach --recursive 'git branch -r --no-merged | grep "[0-9]\{6\}" | while read branch; do echo "$(git log -1 --format=%an $branch)" "$branch"; done | sort' | grep -v Entering | sed -e 's/.*\([0-9]\{6\}\).*/\1/' | egrep '^3'

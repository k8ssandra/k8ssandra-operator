#!/usr/bin/env bash

# Check that wget is available on the system before starting
echo "Checking for wget..."
wgetExists="$(which wget ; echo $?)"
if [[ "$wgetExists" -eq 1 ]]; then
  echo "wget is required and was not found on the system."
  exit 1
fi

tempDir=crd-docs-temp
execName=crd-to-markdown

# Create a temp directory to work within
rm -rf $tempDir
mkdir $tempDir
cd $tempDir
# create a docs directory within the temp directory where the generated files will go
mkdir docs

# Identify the OS that we're on
unameOut="$(uname -s)"
case "${unameOut}" in
    Linux*)     machine=linux;;
    Darwin*)    machine=mac;;
    CYGWIN*)    machine=cygwin;;
    *)          machine="UNKNOWN:${unameOut}"
esac

# Download the binary of the tool for the OS we're on
# This is definitely not fool proof -- wouldn't work for m1 macs as it is for example I don't think
if [ "$machine" = "mac" ]
then
  wget https://github.com/clamoriniere/crd-to-markdown/releases/download/v0.0.3/crd-to-markdown_Darwin_x86_64 -O $execName
elif [ "$machine" = "linux"]
then
  wget https://github.com/clamoriniere/crd-to-markdown/releases/download/v0.0.3/crd-to-markdown_Linux_x86_64 -O $execName
elif [ "$machine" = "cygwin"]
then
  wget https://github.com/clamoriniere/crd-to-markdown/releases/download/v0.0.3/crd-to-markdown_Linux_x86_64 -O $execName
else
  echo "Unsupported OS detected"
  exit 1
fi

chmod +x $execName

# Find all of the _types.go files to be processed
typesArray=( $(find ../../apis -regex '.*_types.go$') )

# For each file found generate the markdown
for file in ${typesArray[@]}
do
  # We need to extract a filename <type> for to be generate <type>_types.go
  if [[ $file =~ .*/(.+)_types.go ]]; then
    type=${BASH_REMATCH[1]}
    echo "Generating $type.md from $file"
    # Do a little manipulation here to cleanup the output -- removing the header we don't want
    (./$execName -f $file | grep -v '### Sub Resources') > docs/$type.md
  else
    echo "Error:  Unable to identify type for $file"
  fi
done

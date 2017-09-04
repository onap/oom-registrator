#!/bin/sh
# force any errors to cause the script and job to end in failure
set -u

DIRNAME=`dirname $0`
RUNHOME=`cd $DIRNAME/; pwd`
# set variable
. $RUNHOME/env.sh

#create workdir
cd $buildDir
rm -rf $workDir
mkdir -p $workDir

cp $dockerDir/build_docker_image.sh  $workDir/
cp $dockerDir/Dockerfile  $workDir/

#build binary
cd $codeDir

export GOPATH=$GOPATH
make clean
make

cp kube2msb $workDir/

#build image
cd $workDir/
chmod a+x *.sh
ls -l

docker rmi $dockerRegistry/$appName:$appVersion
./build_docker_image.sh -n=$dockerRegistry/$appName -v=$appVersion -d=./docker





#!/bin/sh
dockerRegistry=nexus3.onap.org:10003
appName=onap/oom/kube2msb
appVersion=latest

homeDir=$WORKSPACE/kube2msb

buildDir=$homeDir/build
dockerDir=$buildDir/docker
workDir=$buildDir/workDir

GOPATH=$homeDir
codeDir=$GOPATH/src/kube2msb



echo '###########################'
echo @APPNAME@ $appName
echo @APPVersion@ $appVersion
echo @homeDir@ $homeDir
echo @dockerDir@ $dockerDir
echo @workDir@ $workDir
echo @GOPATH@ $GOPATH
echo @codeDir@ $codeDir
echo '###########################'

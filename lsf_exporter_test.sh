#!/usr/bin/env bash
#===============================================================================
#
#          FILE: lsf_exporter_test.sh
#
#         USAGE: ./lsf_exporter_test.sh
#
#   DESCRIPTION:
#
#       OPTIONS: ---
#  REQUIREMENTS: ---
#          BUGS: ---
#         NOTES: ---
#        AUTHOR: Alberto Varesio (JCN), jcn9000@gmail.com
#  ORGANIZATION:
#       CREATED: 11/20/2025 03:47:33 PM
#      REVISION:  ---
#===============================================================================

set -o nounset                              # Treat unset variables as an error

export LSF_TOP=/soft/LSF
export LSF_ENVDIR=/soft/LSF/conf
export LSF_BINDIR=/soft/LSF/10.1/linux2.6-glibc2.3-x86_64/bin
export LSF_LIBDIR=/soft/LSF/10.1/linux2.6-glibc2.3-x86_64/lib
export LSF_SERVERDIR=/soft/LSF/10.1/linux2.6-glibc2.3-x86_64/etc
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/soft/LSF/10.1/linux2.6-glibc2.3-x86_64/bin

## LOGLEVEL=debug ./lsf_exporter --web.listen-address=:9999 --lsf.std-solver-config=/usr/local/etc/Solver-Standard.csv --log.level=debug > run.log 2>&1

/usr/local/bin/lsf_exporter --web.listen-address=:9998 --lsf.std-solver-config=/usr/local/etc/Solver-Standard.csv > /tmp/cur_run.log 2>&1 &
CURPID=$!
             ./lsf_exporter --web.listen-address=:9999 --lsf.std-solver-config=/usr/local/etc/Solver-Standard.csv > /tmp/new_run.log 2>&1 &
NEWPID=$!

sleep 1
curl -s localhost:9998/metrics > /tmp/cur_metric.log
curl -s localhost:9999/metrics > /tmp/new_metric.log

kill $CURPID $NEWPID


# vim:set ai et sts=2 sw=2 tw=80:



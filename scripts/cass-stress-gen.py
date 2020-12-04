#!/usr/bin/env python3
import argparse
import sys
import os

template = """
apiVersion: batch/v1
kind: Job
metadata:
  name: {d.name}-{}
  namespace: {d.namespace}
  labels:
    app: cassandra-stress
spec:
  template:
    spec:
      containers:
      - name: cassandra-stress
        image: scylladb/scylla:{d.scylla_version}
        command:
          - "/bin/bash"
          - "-c"
          - 'cassandra-stress write no-warmup n={d.ops} cl=ONE -mode native cql3 connectionsPerHost={d.connections_per_host} -col n=FIXED\(5\) size=FIXED\(64\)  -pop seq={}..{} -node "{d.host}" -rate threads={d.threads} {d.limit} -log file=/cassandra-stress.load.data -schema "replication(factor=1)" -errors ignore; cat /cassandra-stress.load.data'
        resources:
          limits:
            cpu: {d.cpu}
            memory: {d.memory}
      restartPolicy: Never
      nodeSelector:
        {d.nodeselector}
      tolerations:
        - key: role
          operator: Equal
          value: cassandra-stress
          effect: NoSchedule
      affinity:
        podAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                topologyKey: kubernetes.io/hostname
                labelSelector:
                  matchLabels:
                    app: cassandra-stress
       
"""

def parse():
    parser = argparse.ArgumentParser(description='Generate cassandra-stress job templates for Kubernetes.')
    parser.add_argument('--num-jobs', type=int, default=1, help='number of Kubernetes jobs to generate - defaults to 1', dest='num_jobs')
    parser.add_argument('--name', default='cassandra-stress', help='name of the generated yaml file - defaults to cassandra-stress')
    parser.add_argument('--namespace', default='default', help='namespace of the cassandra-stress jobs - defaults to "default"')
    parser.add_argument('--scylla-version', default='4.0.0', help='version of scylla server to use for cassandra-stress - defaults to 4.0.0', dest='scylla_version')
    parser.add_argument('--host', default='scylla-cluster-client.scylla.svc', help='ip or dns name of host to connect to - defaults to scylla-cluster-client.scylla.svc')
    parser.add_argument('--cpu', default=1, type=int, help='number of cpus that will be used for each job - defaults to 1')
    parser.add_argument('--memory', default=None, help='memory that will be used for each job in GB, ie 2G - defaults to 2G * cpu')
    parser.add_argument('--ops', type=int, default=10000000, help='number of operations for each job - defaults to 10000000')
    parser.add_argument('--threads', default=None, help='number of threads used for each job - defaults to 50 * cpu')
    parser.add_argument('--limit', default='', help='rate limit for each job - defaults to no rate-limiting')
    parser.add_argument('--connections-per-host', default=None, help='number of connections per host - defaults to number of cpus', dest='connections_per_host')
    parser.add_argument('--print-to-stdout', action='store_const', const=True, help='print to stdout instead of writing to a file', dest='print_to_stdout')
    parser.add_argument('--nodeselector', default="", help='nodeselector limits cassandra-stress pods to certain nodes. Use as a label selector, eg. --nodeselector role=scylla')
    return parser.parse_args()

def create_job_list(args):
    manifests = []
    for i in range(args.num_jobs):
        manifests.append(template.format(i, i*args.ops+1, (i+1)*args.ops, d=args))
    return manifests

def get_script_path():
    return os.path.dirname(os.path.realpath(sys.argv[0]))

if __name__ == "__main__":
    args = parse()

    # Fix arguments
    if args.memory is None:
      args.memory = "{}G".format(2*args.cpu)
    if args.threads is None:
      args.threads = 50*args.cpu
    if args.connections_per_host is None:
      args.connections_per_host = args.cpu
    if args.limit:
      args.limit = 'throttle={}/s'.format(args.limit)
    if args.nodeselector:
      parts = args.nodeselector.split("=")
      args.nodeselector = "{}: {}".format(parts[0], parts[1])

    # Create manifests
    manifests = create_job_list(args)
    if args.print_to_stdout:
      print('\n---\n'.join(manifests))
    else:
      f = open(get_script_path() + '/' + args.name + '.yaml', 'w')
      f.write('\n---\n'.join(manifests))
      f.close()

#!/usr/bin/env python3
import argparse

template = """apiVersion: batch/v1
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
          - "bash"
          - "-c"
          - "cassandra-stress {d.cmd} -node {d.host}" 
        resources:
          limits:
            cpu: {d.cpu}
            memory: {d.memory}
      restartPolicy: Never
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
                    app: cassandra-stress"""


def parse():
    parser = argparse.ArgumentParser(description='Generate cassandra-stress job templates for Kubernetes.')
    parser.add_argument('--cmd', required=True, help='cassandra-stress command to run')

    parser.add_argument('--namespace', default='default', help='namespace of the cassandra-stress jobs - defaults to "default"')
    parser.add_argument('--name', default='cassandra-stress', help='name of the generated job')
    parser.add_argument('--num-jobs', type=int, default=1, help='number of Kubernetes jobs to generate - defaults to 1', dest='num_jobs')
    parser.add_argument('--host', default='scylla-cluster-client.scylla.svc', help='ip or dns name of host to connect to - defaults to scylla-cluster-client.scylla.svc')
    parser.add_argument('--cpu', default=1, type=int, help='number of cpus that will be used for each job - defaults to 1')
    parser.add_argument('--memory', default=None, help='memory that will be used for each job in GB, ie 2G - defaults to 2G * cpu')

    parser.add_argument('--scylla-version', default='4.0.0', help='version of scylla server to use for cassandra-stress - defaults to 4.0.0', dest='scylla_version')
    return parser.parse_args()


def create_job_list(args):
    manifests = []
    for i in range(args.num_jobs):
        manifests.append(template.format(i, d=args))
    return manifests


if __name__ == "__main__":
    args = parse()

    # Fix arguments
    if args.memory is None:
      args.memory = "{}G".format(2*args.cpu)

    # Create manifests
    print('\n---\n'.join(create_job_list(args)))

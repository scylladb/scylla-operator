# Deploying Scylla Manager on a Kubernetes Cluster

Scylla Manager is a product for database operations automation, 
it can schedule tasks such as repairs and backups. 
Scylla Manager can manage multiple Scylla clusters and run cluster-wide tasks 
in a controlled and predictable way.

Scylla Manager is available for Scylla Enterprise customers and Scylla Open Source users. 
With Scylla Open Source, Scylla Manager is limited to 5 nodes. 
See the Scylla Manager [Proprietary Software License Agreement](https://www.scylladb.com/scylla-manager-software-license-agreement/) for details.

## Prerequisites

* Kubernetes cluster
* Scylla Operator - see [generic guide](generic.md)

## Architecture

Scylla Manager in K8s consist of:
- Dedicated Scylla Cluster

  Scylla Manager persists its state to a Scylla cluster. 
Additional small single node cluster is spawned in the Manager namespace.  

- Scylla Manager Controller 

  Main mission of Controller is to watch changes of Scylla Clusters, and synchronize three states.
  1. What user wants - task definition in CRD.
  2. What Controller registered - Task name to Task ID mapping - CRD status.
  3. Scylla Manager task listing - internal state of Scylla Manager.
  
  When Scylla Cluster CRD is being deployed Controller will register it in Scylla Manager once cluster reaches desired node count.
Once Cluster is fully up and running it will schedule all tasks defined in Cluster CRD.
Controller also supports task updates and unscheduling.

- Scylla Manager

  Regular Scylla Manager, the same used in cloud and bare metal deployments.

   

## Deploy Scylla Manager

Deploy the Scylla Manager using the following commands:

```console
kubectl apply -f examples/common/manager.yaml
```

This will install the Scylla Manager in the `scylla-manager-system` namespace.
You can check if the Scylla Manager is up and running with:
 
```console
kubectl -n scylla-manager-system get pods                                              
NAME                                               READY   STATUS    RESTARTS   AGE
scylla-manager-cluster-manager-dc-manager-rack-0   2/2     Running   0          37m
scylla-manager-controller-0                        1/1     Running   0          28m
scylla-manager-scylla-manager-7bd9f968b9-w25jw     1/1     Running   0          37m
```
 
As you can see there are three pods:
* `scylla-manager-cluster-manager-dc-manager-rack-0` - is a single node Scylla cluster.
* `scylla-manager-controller-0` - Scylla Manager Controller.
* `scylla-manager-scylla-manager-7bd9f968b9-w25jw` - Scylla Manager.
 
To see if Scylla Manager is fully up and running we can check their logs.
To do this, execute following command:
 
 ```console
kubectl -n scylla-manager-system logs scylla-manager-controller-0 
```

The output should be something like:
```console
{"L":"INFO","T":"2020-09-23T11:25:27.882Z","M":"Scylla Manager Controller started","version":"","build_date":"","commit":"","built_by":"","go_version":"","options":{"Name":"scylla-manager-controller-0","Namespace":"scylla-manager-system","LogLevel":"debug","ApiAddress":"http://127.0.0.1:5080/api/v1"},"_trace_id":"LQEJV3kDR5Gx9M3XQ2YnnQ"}
{"L":"INFO","T":"2020-09-23T11:25:28.435Z","M":"Registering Components.","_trace_id":"LQEJV3kDR5Gx9M3XQ2YnnQ"}
```

To check logs of Scylla Manager itself, use following command:
```console
kubectl -n scylla-manager-system logs scylla-manager-scylla-manager-7bd9f968b9-w25jw
```

The output should be something like:

```console
{"L":"INFO","T":"2020-09-23T11:26:53.238Z","M":"Scylla Manager Server","version":"2.1.2-0.20200816.76cc4dcc","pid":1,"_trace_id":"xQhkJ0OuR8e6iMDEpM62Hg"}
{"L":"INFO","T":"2020-09-23T11:26:54.519Z","M":"Using config","config":{"HTTP":"127.0.0.1:5080","HTTPS":"","TLSCertFile":"/var/lib/scylla-manager/scylla_manager.crt","TLSKeyFile":"/var/lib/scylla-manager/scylla_manager.key","TLSCAFile":"","Prometheus":":56090","PrometheusScrapeInterval":5000000000,"debug":"127.0.0.1:56112","Logger":{"Mode":"stderr","Level":"info","Development":false},"Database":{"Hosts":["scylla-manager-cluster-manager-dc-manager-rack-0.scylla-manager-system.svc"],"SSL":false,"User":"","Password":"","LocalDC":"","Keyspace":"scylla_manager","MigrateDir":"/etc/scylla-manager/cql","MigrateTimeout":30000000000,"MigrateMaxWaitSchemaAgreement":300000000000,"ReplicationFactor":1,"Timeout":600000000,"TokenAware":true},"SSL":{"CertFile":"","Validate":true,"UserCertFile":"","UserKeyFile":""},"Healthcheck":{"Timeout":250000000,"SSLTimeout":750000000},"Backup":{"DiskSpaceFreeMinPercent":10,"AgeMax":43200000000000},"Repair":{"SegmentsPerRepair":1,"ShardParallelMax":0,"ShardFailedSegmentsMax":100,"PollInterval":200000000,"ErrorBackoff":300000000000,"AgeMax":0,"ShardingIgnoreMsbBits":12}},"config_files":["/mnt/etc/scylla-manager/scylla-manager.yaml"],"_trace_id":"xQhkJ0OuR8e6iMDEpM62Hg"}
{"L":"INFO","T":"2020-09-23T11:26:54.519Z","M":"Checking database connectivity...","_trace_id":"xQhkJ0OuR8e6iMDEpM62Hg"}
```

If there are no errors in the logs, let's spin a Scylla Cluster.

## Cluster registration


When the Scylla Manager is fully up and running, lets create a regular instance of Scylla cluster. 

See [generic tutorial](generic.md) to spawn your cluster.

Note: If you already have some Scylla Clusters, after installing Manager they should be 
automatically registered in Scylla Manager.

Once cluster reaches desired node count, cluster status will be updated with ID under which it was registered in Manager.

 ```console
kubectl -n scylla describe Cluster

[...]
Status:
  Manager Id:  d1d532cd-49f2-4c97-9263-25126532803b
  Racks:
    us-east-1a:
      Members:        3
      Ready Members:  3
      Version:        4.0.0
```
You can use this ID to talk to Scylla Manager using `sctool` CLI installed in Scylla Manager Pod.
You can also use Cluster name in `namespace/cluster-name` format.

```console
kubectl -n scylla-manager-system exec -ti scylla-manager-scylla-manager-7bd9f968b9-w25jw -- sctool task list

Cluster: scylla/simple-cluster (d1d532cd-49f2-4c97-9263-25126532803b)
╭─────────────────────────────────────────────────────────────┬──────────────────────────────────────┬────────────────────────────────┬────────╮
│ Task                                                        │ Arguments                            │ Next run                       │ Status │
├─────────────────────────────────────────────────────────────┼──────────────────────────────────────┼────────────────────────────────┼────────┤
│ healthcheck/400b2723-eec5-422a-b7f3-236a0e10575b            │                                      │ 23 Sep 20 14:28:42 CEST (+15s) │ DONE   │
│ healthcheck_rest/28169610-a969-4c20-9d11-ab7568b8a1bd       │                                      │ 23 Sep 20 14:29:57 CEST (+1m)  │ NEW    │
╰─────────────────────────────────────────────────────────────┴──────────────────────────────────────┴────────────────────────────────┴────────╯

```

Scylla Manager by default registers recurring healhcheck tasks for Agent and for each of the enabled frontends (CQL, Alternator).

In this task listing we can see CQL and REST healthchecks.

## Task scheduling

You can either define tasks prior Cluster creation, or for existing Cluster.
Let's edit already running cluster definition to add repair and backup task.
```console
kubectl -n scylla edit Cluster simple-cluster                            
``` 

Add following task definition to Cluster spec:
```
  repairs:
    - name: "users repair"
      keyspace: ["users"]
      interval: "1d"
  backup:
    - name: "weekly backup"
      location: ["s3:cluster-backups"]
      retention: 3
      interval: "7d"
    - name: "daily backup"
      location: ["s3:cluster-backups"]
      retention: 7
      interval: "1d" 
```

For full task definition configuration consult [Scylla Cluster CRD](scylla_cluster_crd.md).

**Note**: Scylla Manager Agent must have access to above bucket prior the update in order to schedule backup task.
Consult Scylla Manager documentation for details on how to set it up.  

Scylla Manager Controller will spot this change and will schedule tasks in Scylla Manager.

```console
kubectl -n scylla-manager-system exec -ti scylla-manager-scylla-manager-7bd9f968b9-w25jw -- sctool task list

Cluster: scylla/simple-cluster (d1d532cd-49f2-4c97-9263-25126532803b)
╭─────────────────────────────────────────────────────────────┬──────────────────────────────────────┬────────────────────────────────┬────────╮
│ Task                                                        │ Arguments                            │ Next run                       │ Status │
├─────────────────────────────────────────────────────────────┼──────────────────────────────────────┼────────────────────────────────┼────────┤
│ healthcheck/400b2723-eec5-422a-b7f3-236a0e10575b            │                                      │ 23 Sep 20 14:28:42 CEST (+15s) │ DONE   │
│ backup/275aae7f-c436-4fc8-bcec-479e65fb8372                 │ -L s3:cluster-backups  --retention 3 │ 23 Sep 20 14:28:58 CEST (+7d)  │ NEW    │
│ healthcheck_rest/28169610-a969-4c20-9d11-ab7568b8a1bd       │                                      │ 23 Sep 20 14:29:57 CEST (+1m)  │ NEW    │
│ repair/d4946360-c29d-4bb4-8b9d-619ada495c2a                 │                                      │ 23 Sep 20 14:38:42 CEST        │ NEW    │
╰─────────────────────────────────────────────────────────────┴──────────────────────────────────────┴────────────────────────────────┴────────╯

```

As you can see, we have two new tasks, weekly recurring backup, and one repair which should start shortly. 

To check progress of run you can use following command:

```console
kubectl -n scylla-manager-system exec -ti scylla-manager-scylla-manager-7bd9f968b9-w25jw -- sctool task progress --cluster d1d532cd-49f2-4c97-9263-25126532803b repair/d4946360-c29d-4bb4-8b9d-619ada495c2a
Status:         RUNNING
Start time:     23 Sep 20 14:38:42 UTC
Duration:       13s
Progress:       2.69%
Datacenters:
  - us-east-1
+--------------------+-------+
| system_auth        | 8.06% |
| system_distributed | 0.00% |
| system_traces      | 0.00% |
+--------------------+-------+

```
Other tasks can be also tracked using the same command, but using different task ID.
Task IDs are present in Cluster Status as well as in task listing.

## Clean Up
 
To clean up all resources associated with Scylla Manager, you can run the commands below.

**NOTE:** this will destroy your Scylla Manager database and delete all of its associated data.

```console
kubectl delete -f examples/common/manager.yaml
```

## Troubleshooting

**Manager is not running**

If the Scylla Manager does not come up, the first step would be to examine the Manager and Controller logs:

```console
kubectl -n scylla-manager-system logs -f scylla-manager-controller-0 scylla-manager-controller
kubectl -n scylla-manager-system logs -f scylla-manager-controller-0 scylla-manager-scylla-manager-7bd9f968b9-w25jw
```


**My task wasn't scheduled**

If your task wasn't scheduled, Cluster status will be updated with error messages for each failed task.
You can also consult Scylla Manager logs.

Example:

Following status describes error when backup task cannot be scheduled, due to lack of access to bucket:
```console
Status:
  Backups:
    Error:     create backup target: location is not accessible: 10.100.16.62: giving up after 2 attempts: after 15s: timeout - make sure the location is correct and credentials are set, to debug SSH to 10.100.16.62 and run "scylla-manager-agent check-location -L s3:manager-test --debug"; 10.107.193.33: giving up after 2 attempts: after 15s: timeout - make sure the location is correct and credentials are set, to debug SSH to 10.107.193.33 and run "scylla-manager-agent check-location -L s3:manager-test --debug"; 10.109.197.60: giving up after 2 attempts: after 15s: timeout - make sure the location is correct and credentials are set, to debug SSH to 10.109.197.60 and run "scylla-manager-agent check-location -L s3:manager-test --debug"
    Id:        00000000-0000-0000-0000-000000000000
    Interval:  0
    Location:
      s3:manager-test
    Name:         adhoc backup
    Num Retries:  3
    Retention:    3
    Start Date:   now
  Manager Id:     2b9dbe8c-9daa-4703-a66d-c29f63a917c8
  Racks:
    us-east-1a:
      Members:        3
      Ready Members:  3
      Version:        4.0.0
```

Because Controller is infinitely retrying to schedule each defined task, once permission issues will be resolved,
task should appear in task listing and Cluster status.
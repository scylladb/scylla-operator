# Scheduler service

### Useful links

- `sctool tasks` command to list all scheduled tasks https://manager.docs.scylladb.com/stable/sctool/task.html
- `sctool info` to see the details about particular task https://manager.docs.scylladb.com/stable/sctool/info.html
- `sctool progress` to see the task execution progress https://manager.docs.scylladb.com/stable/sctool/progress.html

### General picture

![Scheduling](scylla-manager-scheduler.drawio.svg)

Scylla Manager's scheduler service is responsible for proper scheduling of all the maintenance tasks defined for the cluster with scylla-manager.<br/>
Tasks are created in asynchronous way. First, it's the manager REST API consumer to define the task and send it to scylla-manager. Later, it's the scheduler service to trigger its execution according to the cron definition.<br/>

`sctool` is one of the manager's API consumer.<br/>

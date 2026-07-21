# Overview

There are generally 2 types of resources in Kubernetes: [namespaced]() and [cluster-scoped]() resources.

## Namespaced resources


            <div class="topics-grid ">
                
                <div class="grid-container full">
                    <div class="grid-x grid-margin-x">
            
            
                <div class="cell topic-box large-6">
                    <a class="card" href="scyllaclusters/basics.md" >
                
                <div class="topic-box_\_head">
                
            <div class="topic-box_\_icon">
                <i class="icon-database"></i>
            </div>
            
                <h3 class="topic-box_\_title">ScyllaClusters</h3>
                </div>
                <div class="topic-box_\_body">
            
ScyllaCluster defines a **single datacenter** ScyllaDB cluster and manages the racks within.


            
                <div class="topic-box_\_anchor">Learn more »</div>
                
            </div>
            </a></div>
            
            
                <div class="cell topic-box large-6">
                    <a class="card" href="scylladbclusters/scylladbclusters.md" >
                
                <div class="topic-box_\_head">
                
            <div class="topic-box_\_icon">
                <i class="icon-database"></i>
            </div>
            
                <h3 class="topic-box_\_title">ScyllaDBClusters</h3>
                </div>
                <div class="topic-box_\_body">
            
ScyllaDBCluster defines a **multi datacenter** ScyllaDB cluster spanned across multiple Kubernetes clusters and manages the datacenters and racks within.


            
                <div class="topic-box_\_anchor">Learn more »</div>
                
            </div>
            </a></div>
            
            
                <div class="cell topic-box large-6">
                    <a class="card" href="resources/scylladbmonitorings.html" >
                
                <div class="topic-box_\_head">
                
            <div class="topic-box_\_icon">
                <i class="icon-monitoring"></i>
            </div>
            
                <h3 class="topic-box_\_title">ScyllaDBMonitorings</h3>
                </div>
                <div class="topic-box_\_body">
            
ScyllaDBMonitoring defines and manages a ScyllaDB monitoring stack.


            
                <div class="topic-box_\_anchor">Learn more »</div>
                
            </div>
            </a></div>
            </div></div></div>

## Cluster scoped resources

Cluster scoped resources declare the state for the whole cluster or control something at the cluster level which isn’t multitenant.
Therefore, working with these resources requires elevated privileges.

Generally, there can be multiple instances of these resources but for some of them, like for [ScyllaOperatorConfigs](https://operator.docs.scylladb.com/v1.19/resources/scyllaoperatorconfigs.md), it only makes sense to have one instance. We call them *singletons* and the single instance is named `cluster`.


            <div class="topics-grid ">
                
                <div class="grid-container full">
                    <div class="grid-x grid-margin-x">
            
            
                <div class="cell topic-box large-6">
                    <a class="card" href="nodeconfigs.md" >
                
                <div class="topic-box_\_head">
                
            <div class="topic-box_\_icon">
                <i class="icon-tune"></i>
            </div>
            
                <h3 class="topic-box_\_title">NodeConfigs</h3>
                </div>
                <div class="topic-box_\_body">
            
NodeConfig manages setup and tuning for a set of Kubernetes nodes.


            
                <div class="topic-box_\_anchor">Learn more »</div>
                
            </div>
            </a></div>
            
            
                <div class="cell topic-box large-6">
                    <a class="card" href="scyllaoperatorconfigs.md" >
                
                <div class="topic-box_\_head">
                
            <div class="topic-box_\_icon">
                <i class="icon-settings"></i>
            </div>
            
                <h3 class="topic-box_\_title">ScyllaOperatorConfigs</h3>
                </div>
                <div class="topic-box_\_body">
            
ScyllaOperatorConfig configures the Scylla Operator deployment and reports the status.


            
                <div class="topic-box_\_anchor">Learn more »</div>
                
            </div>
            </a></div>
            
            
                <div class="cell topic-box large-6">
                    <a class="card" href="remotekubernetesclusters.md" >
                
                <div class="topic-box_\_head">
                
            <div class="topic-box_\_icon">
                <i class="icon-server-3"></i>
            </div>
            
                <h3 class="topic-box_\_title">RemoteKubernetesClusters</h3>
                </div>
                <div class="topic-box_\_body">
            
RemoteKubernetesCluster configures the access for Scylla Operator to remotely deployed Kubernetes clusters.


            
                <div class="topic-box_\_anchor">Learn more »</div>
                
            </div>
            </a></div>
            </div></div></div>

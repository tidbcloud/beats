# Dev Guide

Please refer to TiKV module's [Dev Guide](../tikv/README.md) for more information.

# TiDB Cluster Components

TiDB operator tags each pod with the [label `app.kubernetes.io/component`](https://github.com/pingcap/tidb-operator/blob/master/pkg/apis/label/label.go#L31).

[Possible components](https://github.com/pingcap/tidb-operator/blob/master/pkg/apis/label/label.go#L122) are:

```text
pd
tidb
tikv
tiflash
ticdc
monitor
clean
restore
backup
dm-master
dm-worker
ng-monitoring
```

These component values are also a part of pod name. Filebeat uses them to discover log files.

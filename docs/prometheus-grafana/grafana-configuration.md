# Visualising cluster metrics using Grafana

Grafana is a tool commonly used to visualise Prometheus metrics. 

MCAC provides a set of [dashboards](https://github.com/datastax/metric-collector-for-apache-cassandra/tree/master/dashboards/grafana/generated-dashboards) designed to surface the most critical Cassandra metrics.

If you've installed the kube-prometheus manifests, you will already have a Grafana instance running in your cluster, which you can port forward to using the below command:

```
kubectl port-forward -n monitoring services/grafana 10000:3000
```

You can then log in using the highly secure `admin`/`admin` username and password combination.

## Importing the Grafana dashboards

If you hover on the `+` icon on the left hand of the Grafana dashboards an option will come up to import a dashboard. click it and copy in one of the dashboards defined in the [MCAC repo](https://github.com/datastax/metric-collector-for-apache-cassandra/blob/master/dashboards/grafana/generated-dashboards).

Follow the prompts and you will be taken to the dashboard, which should be populated with data from Prometheus.


# Grafana operator

While suitable for a quick start guide, the above procedure seems slightly clunky and requires manual intervention to import dashboards. Worst of all, if you delete the pod your dashboards will be lost.

It is worth noting that [Grafana](https://github.com/grafana-operator/grafana-operator) operator allows for [dashboards](https://github.com/grafana-operator/grafana-operator/blob/master/documentation/dashboards.md) to be deployed as yaml manifests, and this is probably the approach you want to opt for in production deployments.
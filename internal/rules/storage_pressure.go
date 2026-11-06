package rules

// storagePressureRules retains target identity: a busy leader must not be
// averaged with idle followers. Event-driven expiration gauges are selected
// only from the current active node; missing gauges remain missing.
func storagePressureRules(recordPrefix, sourcePrefix string) []prometheusRuleItem {
	var result []prometheusRuleItem
	add := func(name, expression string) {
		result = append(result, prometheusRuleItem{
			Record: recordPrefix + ":" + name,
			Expr:   expression,
			Labels: ruleLabels(sourcePrefix),
		})
	}
	active := " and on (cluster, kubernetes_namespace, scrape_profile, pod, instance) (" +
		metricName(sourcePrefix, "core_active") + " == 1)"
	for _, id := range []string{"expire_job_manager_total_jobs", "expire_job_manager_queue_length"} {
		group := targetGrouping()
		if id == "expire_job_manager_queue_length" {
			group = targetGrouping("queue_id")
		}
		add(id+":max", "max by ("+group+") ("+metricName(sourcePrefix, id)+active+")")
	}
	add("expire_job_manager_backlog:net_change_per_second5m",
		`max by (`+targetGrouping()+`) (deriv(`+recordPrefix+
			`:expire_job_manager_total_jobs:max{source_prefix="`+sourcePrefix+`"}[5m]))`)
	add("expire_lease_expiration_time_in_queue:avg5m",
		targetSummaryAverageExpression(sourcePrefix, "expire_lease_expiration_time_in_queue"))
	for _, metric := range []struct{ id, record string }{
		{"raft_storage_put", "raft_storage_put"},
		{"raft_storage_delete", "raft_storage_delete"},
		{"raft_commitTime", "raft_commit_time"},
		{"raft_fsm_applyBatch", "raft_fsm_apply_batch"},
		{"raft_fsm_applyBatchNum", "raft_fsm_apply_batch_logs"},
		{"core_handle_request", "core_handle_request_by_target"},
		{"core_handle_login_request", "core_handle_login_request_by_target"},
	} {
		add(metric.record+":avg5m", targetSummaryAverageExpression(sourcePrefix, metric.id))
	}
	add("raft_thread_fsm_saturation:ratio5m", targetSummaryAverageExpression(sourcePrefix, "raft_thread_fsm_saturation"))
	return result
}

// These summaries contain milliseconds, counts per batch, or busy-wall-time
// fractions depending on the source. Preserve those units and suppress idle
// windows rather than presenting a zero denominator as zero latency.
func targetSummaryAverageExpression(sourcePrefix, id string) string {
	group := targetGrouping()
	sum := "sum by (" + group + ") (rate(" + metricName(sourcePrefix, id+"_sum") + "[5m]))"
	count := "sum by (" + group + ") (rate(" + metricName(sourcePrefix, id+"_count") + "[5m]))"
	return "(" + sum + " / " + count + ") and (" + count + " > 0)"
}

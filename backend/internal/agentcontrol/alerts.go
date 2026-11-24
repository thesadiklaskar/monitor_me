package agentcontrol

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"example.com/monitor-me/backend/internal/telemetry"
)

type alertRuleMatch struct {
	ID        string
	Name      string
	Operator  string
	Threshold float64
}

func evaluateCommittedSamples(ctx context.Context, db *sql.DB, organizationID, serverID string, samples []telemetry.LiveSample) {
	for _, sample := range samples {
		// The metric-name and optional project scope select the relevant sample.
		rows, err := db.QueryContext(ctx,
			"SELECT id::text, name, operator, threshold FROM alert_rules "+
				"WHERE organization_id = $1 AND enabled = true AND metric_name = $2 "+
				"AND (server_id IS NULL OR server_id = $3) "+
				"AND (project_id IS NULL OR project_id = NULLIF($4, '')::uuid) ORDER BY created_at",
			organizationID, sample.MetricName, serverID, sample.ProjectID)
		if err != nil {
			continue
		}
		matches := make([]alertRuleMatch, 0)
		for rows.Next() {
			var rule alertRuleMatch
			if err := rows.Scan(&rule.ID, &rule.Name, &rule.Operator, &rule.Threshold); err != nil {
				matches = nil
				break
			}
			matches = append(matches, rule)
		}
		rowsErr := rows.Err()
		_ = rows.Close()
		if rowsErr != nil {
			continue
		}
		for _, rule := range matches {
			applyAlertRule(ctx, db, organizationID, serverID, sample, rule)
		}
	}
}

func applyAlertRule(ctx context.Context, db *sql.DB, organizationID, serverID string, sample telemetry.LiveSample, rule alertRuleMatch) {
	breached := false
	switch rule.Operator {
	case "gt":
		breached = sample.Value > rule.Threshold
	case "gte":
		breached = sample.Value >= rule.Threshold
	case "lt":
		breached = sample.Value < rule.Threshold
	case "lte":
		breached = sample.Value <= rule.Threshold
	default:
		return
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()

	projectScope := sample.ProjectID
	lockKey := rule.ID + ":" + serverID + ":" + projectScope
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", lockKey); err != nil {
		return
	}
	var eventID string
	err = tx.QueryRowContext(ctx,
		"SELECT id::text FROM alert_events WHERE rule_id = $1 AND organization_id = $2 "+
			"AND server_id = $3 AND project_id IS NOT DISTINCT FROM NULLIF($4, '')::uuid "+
			"AND status = 'firing' FOR UPDATE",
		rule.ID, organizationID, serverID, projectScope).Scan(&eventID)

	if breached {
		if errors.Is(err, sql.ErrNoRows) {
			insertErr := tx.QueryRowContext(ctx,
				"INSERT INTO alert_events (organization_id, rule_id, server_id, project_id, metric_name, status, last_value) "+
					"VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5, 'firing', $6) RETURNING id::text",
				organizationID, rule.ID, serverID, projectScope, sample.MetricName, sample.Value).Scan(&eventID)
			if insertErr != nil {
				return
			}
			message := fmt.Sprintf("%s observed %s; rule is %s %s", sample.MetricName,
				strconv.FormatFloat(sample.Value, 'f', 2, 64), rule.Operator,
				strconv.FormatFloat(rule.Threshold, 'f', 2, 64))
			if err := createAlertNotifications(ctx, tx, organizationID, eventID, "firing", "Alert firing: "+rule.Name, message); err != nil {
				return
			}
		} else if err == nil {
			if _, err := tx.ExecContext(ctx,
				"UPDATE alert_events SET last_value = $1, last_observed_at = now() WHERE id = $2 AND status = 'firing'",
				sample.Value, eventID); err != nil {
				return
			}
		} else {
			return
		}
	} else if err == nil {
		result, err := tx.ExecContext(ctx,
			"UPDATE alert_events SET status = 'resolved', last_value = $1, "+
				"last_observed_at = now(), resolved_at = now() WHERE id = $2 AND status = 'firing'",
			sample.Value, eventID)
		if err != nil {
			return
		}
		count, err := result.RowsAffected()
		if err != nil || count == 0 {
			return
		}
		message := fmt.Sprintf("%s returned below its threshold; latest value is %s",
			sample.MetricName, strconv.FormatFloat(sample.Value, 'f', 2, 64))
		if err := createAlertNotifications(ctx, tx, organizationID, eventID, "recovered", "Alert recovered: "+rule.Name, message); err != nil {
			return
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return
	}
	_ = tx.Commit()
}

func createAlertNotifications(ctx context.Context, tx *sql.Tx, organizationID, eventID, kind, title, message string) error {
	_, err := tx.ExecContext(ctx,
		"INSERT INTO alert_notifications (organization_id, event_id, user_id, kind, title, message) "+
			"SELECT $1, $2, membership.user_id, $3, $4, $5 FROM organization_memberships membership "+
			"WHERE membership.organization_id = $1 ON CONFLICT (event_id, user_id, kind) DO NOTHING",
		organizationID, eventID, kind, title, message)
	return err
}

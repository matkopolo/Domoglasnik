package analytics

import (
	"context"
	"time"
)

type ReleaseSummary struct {
	ID                   string     `json:"id"`
	Status               string     `json:"status"`
	Stage                string     `json:"stage"`
	SourceSaleThrough    *time.Time `json:"sourceSaleThrough,omitempty"`
	SourceRentThrough    *time.Time `json:"sourceRentThrough,omitempty"`
	AnalyticsSaleThrough *time.Time `json:"analyticsSaleThrough,omitempty"`
	AnalyticsRentThrough *time.Time `json:"analyticsRentThrough,omitempty"`
	ModelVersions        *string    `json:"modelVersions,omitempty"`
	StartedAt            time.Time  `json:"startedAt"`
	PublishedAt          *time.Time `json:"publishedAt,omitempty"`
	FailureMessage       *string    `json:"failureMessage,omitempty"`
}

type DataStatus struct {
	Status     string          `json:"status"`
	Active     *ReleaseSummary `json:"activeRelease,omitempty"`
	Pending    *ReleaseSummary `json:"pendingRelease,omitempty"`
	LastFailed *ReleaseSummary `json:"lastFailedRelease,omitempty"`
}

type ReleaseStore interface {
	DataStatus(context.Context) (DataStatus, error)
}

func (r *StatisticsRepository) DataStatus(ctx context.Context) (DataStatus, error) {
	const query = `
		SELECT release.id::text,release.status,release.stage,
		       release.source_sale_through,release.source_rent_through,
		       release.analytics_sale_through,release.analytics_rent_through,
		       release.model_versions,release.started_at,release.published_at,
		       release.failure_message
		FROM analytics.analytical_releases release
		JOIN analytics.analytical_publication publication
		  ON release.id IN (publication.active_release_id,publication.pending_release_id)
		UNION ALL
		SELECT release.id::text,release.status,release.stage,
		       release.source_sale_through,release.source_rent_through,
		       release.analytics_sale_through,release.analytics_rent_through,
		       release.model_versions,release.started_at,release.published_at,
		       release.failure_message
		FROM analytics.analytical_releases release
		JOIN analytics.analytical_publication publication ON publication.singleton
		LEFT JOIN analytics.analytical_releases active
		  ON active.id=publication.active_release_id
		WHERE release.id=(
		    SELECT failed.id FROM analytics.analytical_releases failed
		    WHERE failed.status='failed'
		      AND (active.published_at IS NULL OR failed.failed_at>active.published_at)
		    ORDER BY failed.failed_at DESC LIMIT 1
		)`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return DataStatus{}, err
	}
	defer rows.Close()
	result := DataStatus{Status: "current"}
	for rows.Next() {
		var release ReleaseSummary
		if err := rows.Scan(
			&release.ID, &release.Status, &release.Stage,
			&release.SourceSaleThrough, &release.SourceRentThrough,
			&release.AnalyticsSaleThrough, &release.AnalyticsRentThrough,
			&release.ModelVersions, &release.StartedAt, &release.PublishedAt,
			&release.FailureMessage,
		); err != nil {
			return DataStatus{}, err
		}
		if release.Status == "processing" {
			result.Pending = &release
			result.Status = "processing"
		} else if release.Status == "failed" {
			result.LastFailed = &release
		} else {
			result.Active = &release
		}
	}
	if err := rows.Err(); err != nil {
		return DataStatus{}, err
	}
	if result.Pending != nil {
		result.Status = "processing"
	} else if result.LastFailed != nil {
		result.Status = "failed"
	}
	return result, nil
}

package responder

import (
	"database/sql"

	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"

	"github.com/flanksource/incident-commander/api"
	"github.com/flanksource/incident-commander/db"
)

func getRootHypothesisOfIncident(incidentID uuid.UUID) (api.Hypothesis, error) {
	var hypothesis api.Hypothesis
	if err := db.Gorm.Where("incident_id = ? AND type = ?", incidentID, "root").First(&hypothesis).Error; err != nil {
		return hypothesis, err
	}
	return hypothesis, nil
}

func SyncComments() {
	logger.Debugf("Syncing comments")
	var responders []api.Responder
	err := db.Gorm.Where("external_id IS NOT NULL").Preload("Team").Find(&responders).Error
	if err != nil {
		logger.Errorf("Error fetching responders from database: %v", err)
		return
	}

	dbSelectExternalIDQuery := `
        SELECT external_id FROM comments WHERE responder_id = @responder_id
        UNION
        SELECT external_id FROM comment_responders WHERE responder_id = @responder_id
    `

	var systemUser api.Person
	err = db.Gorm.Where("name = ?", "System").First(&systemUser).Error
	if err != nil {
		logger.Errorf("Error fetching system user from database: %v", err)
		return
	}

	for _, responder := range responders {
		teamSpec, err := responder.Team.GetSpec()
		if err != nil {
			logger.Errorf("Error getting team spec: %v", err)
			continue
		}

		if responder.Properties["responderType"] == JiraResponder {
			jiraClient, err := jiraClientFromTeamSpec(teamSpec)
			if err != nil {
				logger.Errorf("Error instantiating Jira client: %v", err)
				continue
			}

			responderComments, err := jiraClient.GetComments(responder.ExternalID)
			if err != nil {
				logger.Errorf("Error fetching comments from Jira: %v", err)
				continue
			}

			// Query all external_ids from comments and comment_responders table
			var dbExternalIDs []string
			err = db.Gorm.Raw(dbSelectExternalIDQuery, sql.Named("responder_id", responder.ID)).Find(&dbExternalIDs).Error
			if err != nil {
				logger.Errorf("Error querying external_ids from database: %v", err)
				continue
			}

			// IDs which are in Jira but not added to database must be added in the comments table
			for _, responderComment := range responderComments {
				if !collections.Contains(dbExternalIDs, responderComment.ExternalID) {
					rootHypothesis, err := getRootHypothesisOfIncident(responder.IncidentID)
					if err != nil {
						logger.Errorf("Error fetching hypothesis from database: %v", err)
						continue
					}
					responderComment.IncidentID = responder.IncidentID
					responderComment.CreatedBy = systemUser.ID
					responderComment.ResponderID = &responder.ID
					responderComment.HypothesisID = &rootHypothesis.ID

					err = db.Gorm.Create(&responderComment).Error
					if err != nil {
						logger.Errorf("Error inserting comment in database: %v", err)
						continue
					}
				}
			}
		}
	}
}

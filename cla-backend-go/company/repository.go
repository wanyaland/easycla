// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"errors"
	"fmt"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/user"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/strfmt"

	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/communitybridge/easycla/cla-backend-go/gen/models"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/gofrs/uuid"
)

// errors
var (
	ErrCompanyDoesNotExist = errors.New("company does not exist")
)

// IRepository interface methods
type IRepository interface { //nolint
	CreateCompany(in *models.Company) (*models.Company, error)
	GetCompanies() (*models.Companies, error)
	GetCompany(companyID string) (*models.Company, error)
	GetCompanyByExternalID(companySFID string) (*models.Company, error)
	GetCompanyByName(companyName string) (*models.Company, error)
	SearchCompanyByName(companyName string, nextKey string) (*models.Companies, error)
	DeleteCompanyByID(companyID string) error
	DeleteCompanyBySFID(companySFID string) error
	GetCompaniesByUserManager(userID string, userModel user.User) (*models.Companies, error)
	GetCompaniesByUserManagerWithInvites(userID string, userModel user.User) (*models.CompaniesWithInvites, error)

	AddPendingCompanyInviteRequest(companyID string, userID string) (*Invite, error)
	GetCompanyInviteRequest(companyInviteID string) (*Invite, error)
	GetCompanyInviteRequests(companyID string, status *string) ([]Invite, error)
	GetCompanyUserInviteRequests(companyID string, userID string) (*Invite, error)
	GetUserInviteRequests(userID string) ([]Invite, error)
	ApproveCompanyAccessRequest(companyInviteID string) error
	RejectCompanyAccessRequest(companyInviteID string) error
	updateInviteRequestStatus(companyInviteID, status string) error

	UpdateCompanyAccessList(companyID string, companyACL []string) error
}

type repository struct {
	stage                   string
	dynamoDBClient          *dynamodb.DynamoDB
	companyTableName        string
	companyInvitesTableName string
}

// NewRepository creates a new company repository instance
func NewRepository(awsSession *session.Session, stage string) IRepository {
	return repository{
		stage:                   stage,
		dynamoDBClient:          dynamodb.New(awsSession),
		companyTableName:        fmt.Sprintf("cla-%s-companies", stage),
		companyInvitesTableName: fmt.Sprintf("cla-%s-company-invites", stage),
	}
}

// GetCompanies retrieves all the companies
func (repo repository) GetCompanies() (*models.Companies, error) {
	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithProjection(buildCompanyProjection()).Build()
	if err != nil {
		log.Warnf("error building expression for get all companies scan error: %v", err)
		return nil, err
	}

	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyTableName),
	}

	var lastEvaluatedKey string
	var companies []models.Company

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, dbErr := repo.dynamoDBClient.Scan(scanInput)
		if dbErr != nil {
			log.Warnf("error retrieving get all companies, error: %v", dbErr)
			return nil, dbErr
		}

		// Convert the list of DB models to a list of response models
		companyList, modelErr := buildCompanyModels(results)
		if modelErr != nil {
			log.Warnf("error retrieving get all companies, error: %v", modelErr)
			return nil, modelErr
		}

		// Add to our response model list
		companies = append(companies, companyList...)

		if results.LastEvaluatedKey["company_id"] != nil {
			//log.Debugf("LastEvaluatedKey: %+v", result.LastEvaluatedKey["signature_id"])
			lastEvaluatedKey = *results.LastEvaluatedKey["company_id"].S
			scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"company_id": {
					S: aws.String(lastEvaluatedKey),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}
	}

	// How many total records do we have - may not be up-to-date as this value is updated only periodically
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: &repo.companyTableName,
	}

	describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
	if err != nil {
		log.Warnf("error retrieving total company record count, error: %v", err)
		return nil, err
	}

	totalCount := *describeTableResult.Table.ItemCount

	return &models.Companies{
		ResultCount:    int64(len(companies)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Companies:      companies,
	}, nil
}

// GetCompanyByExternalID returns a company based on the company external ID
func (repo repository) GetCompanyByExternalID(companySFID string) (*models.Company, error) {
	condition := expression.Key("company_external_id").Equal(expression.Value(companySFID))
	builder := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildCompanyProjection())
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.companyTableName),
		IndexName:                 aws.String("external-company-index"),
	}

	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.Warnf("error retrieving company using company_external_id. error = %s", err.Error())
		return nil, err
	}

	if len(results.Items) == 0 {
		return nil, ErrCompanyDoesNotExist
	}
	dbCompanyModel := DBModel{}
	err = dynamodbattribute.UnmarshalMap(results.Items[0], &dbCompanyModel)
	if err != nil {
		return nil, err
	}
	return dbCompanyModel.toModel()
}

// GetCompanyByName searches the database and returns the matching company names
func (repo repository) GetCompanyByName(companyName string) (*models.Company, error) {
	// This is the key we want to match
	condition := expression.Key("company_name").Equal(expression.Value(companyName))

	// Use the builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildCompanyProjection()).Build()
	if err != nil {
		log.Warnf("error building expression for company query, companyName: %s, error: %v",
			companyName, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyTableName),
		IndexName:                 aws.String("company-name-index"),
	}

	// Make the DynamoDB Query API call
	results, queryErr := repo.dynamoDBClient.Query(queryInput)
	if queryErr != nil {
		log.Warnf("error retrieving company by companyName: %s, error: %+v", companyName, queryErr)
		return nil, queryErr
	}

	// Didn't find it...
	if *results.Count == 0 {
		log.Debugf("Company query by name returned no results using companyName: %s", companyName)
		return nil, nil
	}

	// Found it...
	var dbModels []DBModel
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &dbModels)
	if err != nil {
		log.Warnf("error unmarshalling db company, error: %+v", err)
		return nil, err
	}
	// TODO: DAD - review projection and unmarshalling logic, the 'note' column is not being loaded into the data model
	//log.Debugf("DB response model: %#v", dbModels)

	return toSwaggerModel(&dbModels[0])
}

// GetCompany returns a company based on the company ID
func (repo repository) GetCompany(companyID string) (*models.Company, error) {
	companyTableData, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(repo.companyTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"company_id": {
				S: aws.String(companyID),
			},
		},
	})

	if err != nil {
		log.Warnf(err.Error())
		log.Warnf("error fetching company table data using company id: %s, error: %v", companyID, err)
		return nil, err
	}

	if len(companyTableData.Item) == 0 {
		return nil, ErrCompanyDoesNotExist
	}

	dbCompanyModel := DBModel{}
	err = dynamodbattribute.UnmarshalMap(companyTableData.Item, &dbCompanyModel)
	if err != nil {
		log.Warnf("error unmarshalling company table data, error: %v", err)
		return nil, err
	}
	return dbCompanyModel.toModel()
}

// SearchCompanyByName locates companies by the matching name and return any potential matches
func (repo repository) SearchCompanyByName(companyName string, nextKey string) (*models.Companies, error) {
	// Sorry, no results if empty company name
	if strings.TrimSpace(companyName) == "" {
		return &models.Companies{
			Companies:      []models.Company{},
			LastKeyScanned: "",
			ResultCount:    0,
			SearchTerms:    companyName,
			TotalCount:     0,
		}, nil
	}

	// This is the company name we want to match
	filter := expression.Name("company_name").Contains(companyName)

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithFilter(filter).WithProjection(buildCompanyProjection()).Build()
	if err != nil {
		log.Warnf("error building expression for company scan, companyName: %s, error: %v",
			companyName, err)
		return nil, err
	}

	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyTableName),
	}

	// If we have the next key, set the exclusive start key value
	if nextKey != "" {
		log.Debugf("Received a nextKey, value: %s", nextKey)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"company_id": {
				S: aws.String(nextKey),
			},
		}
	}

	//log.Debugf("Running company search scan using queryInput: %+v", scanInput)

	var lastEvaluatedKey string
	var companies []models.Company

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, dbErr := repo.dynamoDBClient.Scan(scanInput)
		if dbErr != nil {
			log.Warnf("error retrieving companies for search term: %s, error: %v", companyName, dbErr)
			return nil, dbErr
		}

		// Convert the list of DB models to a list of response models
		companyList, modelErr := buildCompanyModels(results)
		if modelErr != nil {
			log.Warnf("error retrieving companies for companyName %s in ACL, error: %v", companyName, modelErr)
			return nil, modelErr
		}

		// Add to our response model list
		companies = append(companies, companyList...)

		if results.LastEvaluatedKey["company_id"] != nil {
			//log.Debugf("LastEvaluatedKey: %+v", result.LastEvaluatedKey["signature_id"])
			lastEvaluatedKey = *results.LastEvaluatedKey["company_id"].S
			scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"company_id": {
					S: aws.String(lastEvaluatedKey),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}
	}

	// How many total records do we have - may not be up-to-date as this value is updated only periodically
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: &repo.companyTableName,
	}

	describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
	if err != nil {
		log.Warnf("error retrieving total company record count for companyName: %s, error: %v", companyName, err)
		return nil, err
	}

	totalCount := *describeTableResult.Table.ItemCount

	return &models.Companies{
		ResultCount:    int64(len(companies)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Companies:      companies,
	}, nil
}

// DeleteCompanyByID deletes the company by ID
func (repo repository) DeleteCompanyByID(companyID string) error {
	_, err := repo.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"company_id": {S: aws.String(companyID)},
		},
		TableName: aws.String(repo.companyTableName),
	})

	if err != nil {
		return err
	}

	return nil
}

// DeleteCompanyBySFID deletes the company by SFID
func (repo repository) DeleteCompanyBySFID(companySFID string) error {
	_, err := repo.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"company_external_id": {S: aws.String(companySFID)},
		},
		TableName: aws.String(repo.companyTableName),
	})

	if err != nil {
		return err
	}

	return nil
}

// GetCompanyUserManager the get a list of companies when provided the company id and user manager
func (repo repository) GetCompaniesByUserManager(userID string, userModel user.User) (*models.Companies, error) {
	// Sorry, no results if empty user ID
	if strings.TrimSpace(userID) == "" {
		return &models.Companies{
			Companies:      []models.Company{},
			LastKeyScanned: "",
			ResultCount:    0,
			TotalCount:     0,
		}, nil
	}

	// This is the user name we want to match
	var filter expression.ConditionBuilder
	if userModel.LFUsername != "" {
		filter = expression.Name("company_acl").Contains(userModel.LFUsername)
	} else if userModel.UserName != "" {
		filter = expression.Name("company_acl").Contains(userModel.UserName)
	} else {
		log.Warnf("unable to query user with no LF username or username in their data model - user iD: %s.", userID)
		return &models.Companies{
			Companies:      []models.Company{},
			LastKeyScanned: "",
			ResultCount:    0,
			TotalCount:     0,
		}, nil
	}

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithFilter(filter).WithProjection(buildCompanyProjection()).Build()
	if err != nil {
		log.Warnf("error building expression for company scan, userID %s in ACL, error: %v", userID, err)
		return nil, err
	}

	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyTableName),
	}

	//log.Debugf("Running company search scan using queryInput: %+v", scanInput)
	var lastEvaluatedKey string
	var companies []models.Company

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		// Make the DynamoDB Query API call
		results, dbErr := repo.dynamoDBClient.Scan(scanInput)
		if dbErr != nil {
			log.Warnf("error retrieving companies for userID %s in ACL, error: %v", userID, dbErr)
			return nil, dbErr
		}

		// Convert the list of DB models to a list of response models
		companyList, modelErr := buildCompanyModels(results)
		if modelErr != nil {
			log.Warnf("error retrieving companies for userID %s in ACL, error: %v", userID, modelErr)
			return nil, modelErr
		}

		// Add to our response model list
		companies = append(companies, companyList...)

		if results.LastEvaluatedKey["company_invite_id"] != nil {
			//log.Debugf("LastEvaluatedKey: %+v", result.LastEvaluatedKey["signature_id"])
			lastEvaluatedKey = *results.LastEvaluatedKey["company_invite_id"].S
			scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"company_invite_id": {
					S: aws.String(lastEvaluatedKey),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}
	}

	// How many total records do we have - may not be up-to-date as this value is updated only periodically
	describeTableInput := &dynamodb.DescribeTableInput{
		TableName: &repo.companyTableName,
	}

	describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
	if err != nil {
		log.Warnf("error retrieving total company record count, error: %v", err)
		return nil, err
	}

	totalCount := *describeTableResult.Table.ItemCount

	return &models.Companies{
		ResultCount:    int64(len(companies)),
		TotalCount:     totalCount,
		LastKeyScanned: lastEvaluatedKey,
		Companies:      companies,
	}, nil
}

// GetCompanyUserManagerWithInvites the get a list of companies including status when provided the company id and user manager
func (repo repository) GetCompaniesByUserManagerWithInvites(userID string, userModel user.User) (*models.CompaniesWithInvites, error) {
	companies, err := repo.GetCompaniesByUserManager(userID, userModel)
	if err != nil {
		log.Warnf("error retrieving companies for userID %s in ACL, error: %v", userID, err)
		return nil, err
	}

	// Query the invites table for list of invitations for this user
	invites, err := repo.GetUserInviteRequests(userID)
	if err != nil {
		log.Warnf("error retrieving companies invites for userID %s, error: %v", userID, err)
		return nil, err
	}

	return repo.buildCompaniesByUserManagerWithInvites(companies, invites), nil
}

func (repo repository) buildCompaniesByUserManagerWithInvites(companies *models.Companies, invites []Invite) *models.CompaniesWithInvites {
	companiesWithInvites := models.CompaniesWithInvites{
		ResultCount: int64(len(companies.Companies) + len(invites)),
		TotalCount:  companies.TotalCount + int64(len(invites)),
	}

	var companyWithInvite []models.CompanyWithInvite
	for _, company := range companies.Companies {
		companyWithInvite = append(companyWithInvite, models.CompanyWithInvite{
			CompanyName:       company.CompanyName,
			CompanyID:         company.CompanyID,
			CompanyExternalID: company.CompanyExternalID,
			CompanyACL:        company.CompanyACL,
			Created:           company.Created,
			Updated:           company.Updated,
			Status:            "Joined",
		})
	}

	for _, invite := range invites {
		company, err := repo.GetCompany(invite.RequestedCompanyID)
		if err != nil {
			log.Warnf("error retrieving company with company ID %s, error: %v - skipping invite", company, err)
			continue
		}

		// Default status is pending if there's a record but no status
		if invite.Status == "" {
			invite.Status = StatusPending
		}

		companyWithInvite = append(companyWithInvite, models.CompanyWithInvite{
			CompanyName: company.CompanyName,
			CompanyID:   company.CompanyID,
			CompanyACL:  company.CompanyACL,
			Created:     company.Created,
			Updated:     company.Updated,
			Status:      invite.Status,
		})
	}

	companiesWithInvites.CompaniesWithInvites = companyWithInvite

	return &companiesWithInvites
}

// buildCompanyModels converts the response model into a response data model
func buildCompanyModels(results *dynamodb.ScanOutput) ([]models.Company, error) {
	var companies []models.Company

	type ItemSignature struct {
		CompanyID         string   `json:"company_id"`
		CompanyName       string   `json:"company_name"`
		CompanyACL        []string `json:"company_acl"`
		CompanyExternalID string   `json:"company_external_id"`
		Created           string   `json:"date_created"`
		Modified          string   `json:"date_modified"`
	}

	// The DB company model
	var dbCompanies []ItemSignature

	err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &dbCompanies)
	if err != nil {
		log.Warnf("error unmarshalling companies from database, error: %v", err)
		return nil, err
	}

	now, _ := utils.CurrentTime()

	for _, dbCompany := range dbCompanies {
		createdDateTime, err := utils.ParseDateTime(dbCompany.Created)
		if err != nil {
			log.Warnf("Unable to parse company created date time: %s, error: %v - using current time",
				dbCompany.Created, err)
			createdDateTime = now
		}

		modifiedDateTime, err := utils.ParseDateTime(dbCompany.Modified)
		if err != nil {
			log.Warnf("Unable to parse company modified date time: %s, error: %v - using current time",
				dbCompany.Created, err)
			modifiedDateTime = now
		}

		companies = append(companies, models.Company{
			CompanyACL:        dbCompany.CompanyACL,
			CompanyID:         dbCompany.CompanyID,
			CompanyName:       dbCompany.CompanyName,
			CompanyExternalID: dbCompany.CompanyExternalID,
			Created:           strfmt.DateTime(createdDateTime),
			Updated:           strfmt.DateTime(modifiedDateTime),
		})
	}

	return companies, nil
}

// GetCompanyInviteRequest returns the specified request
func (repo repository) GetCompanyInviteRequest(companyInviteID string) (*Invite, error) {

	condition := expression.Key("company_invite_id").Equal(expression.Value(companyInviteID))

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildInvitesProjection()).Build()

	if err != nil {
		log.Warnf("error building expression for company invites, invite ID: %s, error: %v", companyInviteID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyInvitesTableName),
	}

	queryResults, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.Warnf("Unable to query the company invite based on invite ID: %s, error: %v", companyInviteID, err)
		return nil, err
	}

	var companyInvites []Invite
	err = dynamodbattribute.UnmarshalListOfMaps(queryResults.Items, &companyInvites)
	if err != nil || companyInvites == nil {
		log.Warnf("unable to unmarshall the company invite based on invite ID: %s, error: %v", companyInviteID, err)
		return nil, err
	}
	if len(companyInvites) == 0 {
		log.Warnf("unable to locate the company invite based on invite ID: %s, error: %v", companyInviteID, err)
		return nil, nil
	}

	return &companyInvites[0], nil
}

// GetCompanyInviteRequests returns a list of company invites when provided the company ID
func (repo repository) GetCompanyInviteRequests(companyID string, status *string) ([]Invite, error) {

	// These are the keys we want to match
	condition := expression.Key("requested_company_id").Equal(expression.Value(companyID))

	// Use the nice builder to create the expression
	builder := expression.NewBuilder().
		WithKeyCondition(condition).
		WithProjection(buildInvitesProjection())

	if status != nil {
		builder.WithFilter(expression.Name("status").Equal(expression.Value(*status)))
	}

	expr, err := builder.Build()
	if err != nil {
		log.Warnf("error building expression for company invite query, companyID: %s, error: %v",
			companyID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyInvitesTableName),
		IndexName:                 aws.String("requested-company-index"), // Name of a secondary index
	}

	companyInviteAV, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.Warnf("Unable to retrieve data from Company-Invites table, error: %v", err)
		return nil, err
	}

	var companyInvites []Invite
	err = dynamodbattribute.UnmarshalListOfMaps(companyInviteAV.Items, &companyInvites)
	if err != nil {
		log.Warnf("error unmarshalling company invite data, error: %v", err)
		return nil, err
	}

	return companyInvites, nil
}

// GetCompanyUserInviteRequests returns a list of company invites when provided the company ID and user ID
func (repo repository) GetCompanyUserInviteRequests(companyID string, userID string) (*Invite, error) {

	// These are the keys we want to match
	condition := expression.Key("requested_company_id").Equal(expression.Value(companyID))
	filter := expression.Name("user_id").Equal(expression.Value(userID))

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().
		WithKeyCondition(condition).
		WithFilter(filter).
		WithProjection(buildInvitesProjection()).Build()
	if err != nil {
		log.Warnf("error building expression for company scan, companyID: %s with userID: %s, error: %v",
			companyID, userID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyInvitesTableName),
		IndexName:                 aws.String("requested-company-index"), // Name of a secondary index
	}

	queryResults, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.Warnf("Unable to retrieve data from Company-Invites table using company id: %s and user id: %s, error: %v", companyID, userID, err)
		return nil, err
	}

	var companyInvites []Invite
	err = dynamodbattribute.UnmarshalListOfMaps(queryResults.Items, &companyInvites)
	if err != nil {
		log.Warnf("error unmarshalling company invite data using company id: %s and user id: %s, error: %v",
			companyID, userID, err)
		return nil, err
	}

	if len(companyInvites) == 0 {
		log.Debugf("Unable to find company invite for company id: %s and user id: %s", companyID, userID)
		return nil, nil
	}

	if len(companyInvites) > 1 {
		log.Warnf("Company invite should have one result, found: %d for company id: %s and user id: %s",
			len(companyInvites), companyID, userID)
	}

	return &companyInvites[0], nil
}

// GetUserInviteRequests returns a list of company invites when provided the user ID
func (repo repository) GetUserInviteRequests(userID string) ([]Invite, error) {

	filter := expression.Name("user_id").Equal(expression.Value(userID))

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().
		WithFilter(filter).
		WithProjection(buildInvitesProjection()).Build()
	if err != nil {
		log.Warnf("error building expression for company scan with userID: %s, error: %v", userID, err)
		return nil, err
	}

	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.companyInvitesTableName),
	}

	var lastEvaluatedKey string
	var companyInvites []Invite

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {

		queryResults, err := repo.dynamoDBClient.Scan(scanInput)
		if err != nil {
			log.Warnf("Unable to retrieve data from Company-Invites table using user id: %s, error: %v", userID, err)
			return nil, err
		}

		var companyInvitesList []Invite
		err = dynamodbattribute.UnmarshalListOfMaps(queryResults.Items, &companyInvitesList)
		if err != nil {
			log.Warnf("error unmarshalling company invite data using user id: %s, error: %v", userID, err)
			return nil, err
		}

		// Add to our response model
		companyInvites = append(companyInvites, companyInvitesList...)

		// Determine if we have more records - if so, update the start key and loop again
		if queryResults.LastEvaluatedKey["company_invite_id"] != nil {
			//log.Debugf("LastEvaluatedKey: %+v", result.LastEvaluatedKey["signature_id"])
			lastEvaluatedKey = *queryResults.LastEvaluatedKey["company_invite_id"].S
			scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"company_invite_id": {
					S: aws.String(lastEvaluatedKey),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}
	}

	return companyInvites, nil
}

// AddPendingCompanyInviteRequest adds a pending company invite when provided the company ID and user ID
func (repo repository) AddPendingCompanyInviteRequest(companyID string, userID string) (*Invite, error) {

	// First, let's check if we already have a previous invite for this company and user ID pair
	previousInvite, err := repo.GetCompanyUserInviteRequests(companyID, userID)
	if err != nil {
		log.Warnf("Previous invite already exists for company id: %s and user: %s, error: %v",
			companyID, userID, err)
		return nil, err
	}

	// We we already have an invite...don't create another one
	if previousInvite != nil {
		log.Warnf("Invite already exists for company id: %s and user: %s - skipping creation", companyID, userID)
		return previousInvite, nil
	}

	companyInviteID, err := uuid.NewV4()
	if err != nil {
		log.Warnf("Unable to generate a UUID for a pending invite, error: %v", err)
		return nil, err
	}

	_, now := utils.CurrentTime()

	input := &dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			"company_invite_id": {
				S: aws.String(companyInviteID.String()),
			},
			"requested_company_id": {
				S: aws.String(companyID),
			},
			"user_id": {
				S: aws.String(userID),
			},
			"status": {
				S: aws.String("pending"),
			},
			"date_created": {
				S: aws.String(now),
			},
			"date_modified": {
				S: aws.String(now),
			},
		},
		TableName: aws.String(fmt.Sprintf("cla-%s-company-invites", repo.stage)),
	}

	_, err = repo.dynamoDBClient.PutItem(input)
	if err != nil {
		log.Warnf("Unable to create a new pending invite, error: %v", err)
		return nil, err
	}

	createdInvite, err := repo.GetCompanyInviteRequest(companyInviteID.String())
	if err != nil || createdInvite == nil {
		log.Warnf("Unable to query newly created company invite id: %s, error: %v",
			companyInviteID.String(), err)
		return nil, err
	}

	return createdInvite, nil
}

// ApproveCompanyAccessRequest approves the specified company invite
func (repo repository) ApproveCompanyAccessRequest(companyInviteID string) error {
	return repo.updateInviteRequestStatus(companyInviteID, "approved")
}

// RejectCompanyInviteRequest rejects the specified company invite
func (repo repository) RejectCompanyAccessRequest(companyInviteID string) error {
	return repo.updateInviteRequestStatus(companyInviteID, "rejected")
}

// updateInviteRequestStatus updates the specified invite with the specified status
func (repo repository) updateInviteRequestStatus(companyInviteID, status string) error {

	// First, let's check if we already have a previous invite
	inviteModel, err := repo.GetCompanyInviteRequest(companyInviteID)
	if err != nil || inviteModel == nil {
		log.Warnf("ApproveCompanyAccessRequest - unable to locate previous invite, error: %v",
			err)
		return err
	}

	_, now := utils.CurrentTime()

	input := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"company_invite_id": {
				S: aws.String(companyInviteID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#C": aws.String("requested_company_id"),
			"#U": aws.String("user_id"),
			"#S": aws.String("status"),
			"#M": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":c": {
				S: aws.String(inviteModel.RequestedCompanyID),
			},
			":u": {
				S: aws.String(inviteModel.UserID),
			},
			":s": {
				S: aws.String(status),
			},
			":m": {
				S: aws.String(now),
			},
		},
		UpdateExpression: aws.String("SET #C = :c, #U = :u, #S = :s, #M = :m"),
		TableName:        aws.String(fmt.Sprintf("cla-%s-company-invites", repo.stage)),
	}

	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.Warnf("ApproveCompanyAccessRequest - unable to update request with approved status, error: %v",
			updateErr)
		return updateErr
	}

	return nil
}

// UpdateCompanyAccessList updates the company ACL when provided the company ID and ACL list
func (repo repository) UpdateCompanyAccessList(companyID string, companyACL []string) error {
	_, now := utils.CurrentTime()

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#S": aws.String("company_acl"),
			"#M": aws.String("date_modified"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":s": {
				SS: aws.StringSlice(companyACL),
			},
			":m": {
				S: aws.String(now),
			},
		},
		TableName: aws.String(repo.companyTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"company_id": {
				S: aws.String(companyID),
			},
		},
		UpdateExpression: aws.String("SET #S = :s, #M = :m"),
	}

	_, err := repo.dynamoDBClient.UpdateItem(input)
	if err != nil {
		log.Warnf("Error updating Company Access List, error: %v", err)
		return err
	}

	return nil
}

// CreateCompany creates a new company record
func (repo repository) CreateCompany(in *models.Company) (*models.Company, error) {
	companyID, err := uuid.NewV4()
	if err != nil {
		log.Warnf("Unable to generate a UUID for a pending invite, error: %v", err)
		return nil, err
	}
	_, now := utils.CurrentTime()
	comp := &DBModel{
		CompanyID:         companyID.String(),
		CompanyName:       in.CompanyName,
		CompanyACL:        in.CompanyACL,
		CompanyExternalID: in.CompanyExternalID,
		CompanyManagerID:  in.CompanyManagerID,
		Created:           now,
		Updated:           now,
		Version:           "v1",
	}
	av, err := dynamodbattribute.MarshalMap(&comp)
	if err != nil {
		return nil, err
	}
	_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(repo.companyTableName),
	})
	if err != nil {
		return nil, err
	}
	log.Debugf("company created %#v\n", comp)
	return comp.toModel()
}

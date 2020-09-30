// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github_organizations

import (
	"context"
	"fmt"
	"strconv"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	v1GithubOrg "github.com/communitybridge/easycla/cla-backend-go/github_organizations"
	v1Repositories "github.com/communitybridge/easycla/cla-backend-go/repositories"
	v2ProjectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"
	"github.com/jinzhu/copier"
)

func v2GithubOrgnizationsModel(in *v1Models.GithubOrganizations) (*models.GithubOrganizations, error) {
	var response models.GithubOrganizations
	err := copier.Copy(&response, in)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func v2GithubOrgnizationModel(in *v1Models.GithubOrganization) (*models.GithubOrganization, error) {
	var response models.GithubOrganization
	err := copier.Copy(&response, in)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// Service contains functions of GithubOrganizations service
type Service interface {
	GetGithubOrganizations(ctx context.Context, projectSFID string) (*models.ProjectGithubOrganizations, error)
	AddGithubOrganization(ctx context.Context, projectSFID string, input *models.CreateGithubOrganization) (*models.GithubOrganization, error)
	DeleteGithubOrganization(ctx context.Context, projectSFID string, githubOrgName string) error
	UpdateGithubOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, branchProtectionEnabled bool) error
}

type service struct {
	repo         v1GithubOrg.Repository
	ghRepository v1Repositories.Repository
}

// NewService creates a new githubOrganizations service
func NewService(repo v1GithubOrg.Repository, ghRepository v1Repositories.Repository) Service {
	return service{
		repo:         repo,
		ghRepository: ghRepository,
	}
}

const (
	// Connected status
	Connected = "connected"
	// PartialConnection status
	PartialConnection = "partial_connection"
	// ConnectionFailure status
	ConnectionFailure = "connection_failure"
	// NoConnection status
	NoConnection = "no_connection"
)

func (s service) GetGithubOrganizations(ctx context.Context, projectSFID string) (*models.ProjectGithubOrganizations, error) {
	psc := v2ProjectService.GetClient()
	_, err := psc.GetProject(projectSFID)
	if err != nil {
		return nil, err
	}

	orgs, err := s.repo.GetGithubOrganizations(ctx, "", projectSFID)
	if err != nil {
		return nil, err
	}
	out := &models.ProjectGithubOrganizations{
		List: make([]*models.ProjectGithubOrganization, 0),
	}
	type githubRepoInfo struct {
		orgName  string
		repoInfo *v1Models.GithubRepositoryInfo
	}
	// connectedRepo contains list of repositories for which github app have permission
	connectedRepo := make(map[string]*githubRepoInfo)
	orgmap := make(map[string]*models.ProjectGithubOrganization)
	for _, org := range orgs.List {
		for _, repoInfo := range org.Repositories.List {
			key := fmt.Sprintf("%s#%v", org.OrganizationName, repoInfo.RepositoryGithubID)
			connectedRepo[key] = &githubRepoInfo{
				orgName:  org.OrganizationName,
				repoInfo: repoInfo,
			}
		}
		rorg := &models.ProjectGithubOrganization{
			AutoEnabled:            org.AutoEnabled,
			ConnectionStatus:       "",
			GithubOrganizationName: org.OrganizationName,
			Repositories:           make([]*models.ProjectGithubRepository, 0),
		}
		orgmap[org.OrganizationName] = rorg
		out.List = append(out.List, rorg)
		if org.OrganizationInstallationID == 0 {
			rorg.ConnectionStatus = NoConnection
		} else {
			if org.Repositories.Error != "" {
				rorg.ConnectionStatus = ConnectionFailure
			} else {
				rorg.ConnectionStatus = Connected
			}
		}
	}
	repos, err := s.ghRepository.ListProjectRepositories(ctx, "", projectSFID, true)
	if err != nil {
		return nil, err
	}
	for _, repo := range repos.List {
		rorg, ok := orgmap[repo.RepositoryOrganizationName]
		if !ok {
			log.Warnf("repositories table contain stale data for organization %s", repo.RepositoryOrganizationName)
			continue
		}
		key := fmt.Sprintf("%s#%v", repo.RepositoryOrganizationName, repo.RepositoryExternalID)
		if _, ok := connectedRepo[key]; ok {
			repoGithubID, err := strconv.ParseInt(repo.RepositoryExternalID, 10, 64)
			if err != nil {
				log.Warnf("repository github id is not integer. error = %s", err)
			}
			rorg.Repositories = append(rorg.Repositories, &models.ProjectGithubRepository{
				ConnectionStatus:   Connected,
				Enabled:            true,
				RepositoryID:       repo.RepositoryID,
				RepositoryName:     repo.RepositoryName,
				RepositoryGithubID: repoGithubID,
			})
			// delete it from connectedRepo array since we have processed it
			// connectedArray after this loop will contain repo for which github app have permission but
			// they are enabled in cla
			delete(connectedRepo, key)
		} else {
			rorg.Repositories = append(rorg.Repositories, &models.ProjectGithubRepository{
				ConnectionStatus: ConnectionFailure,
				Enabled:          true,
				RepositoryID:     repo.RepositoryID,
				RepositoryName:   repo.RepositoryName,
			})
			if rorg.ConnectionStatus == Connected {
				rorg.ConnectionStatus = PartialConnection
			}
		}
	}
	for _, notEnabledRepo := range connectedRepo {
		rorg, ok := orgmap[notEnabledRepo.orgName]
		if !ok {
			log.Warnf("failed to get org %s", notEnabledRepo.orgName)
			continue
		}
		rorg.Repositories = append(rorg.Repositories, &models.ProjectGithubRepository{
			ConnectionStatus:   Connected,
			Enabled:            false,
			RepositoryID:       "",
			RepositoryName:     notEnabledRepo.repoInfo.RepositoryName,
			RepositoryGithubID: notEnabledRepo.repoInfo.RepositoryGithubID,
		})
	}
	return out, nil
}

func (s service) AddGithubOrganization(ctx context.Context, projectSFID string, input *models.CreateGithubOrganization) (*models.GithubOrganization, error) {
	var in v1Models.CreateGithubOrganization
	err := copier.Copy(&in, input)
	if err != nil {
		return nil, err
	}
	psc := v2ProjectService.GetClient()
	project, err := psc.GetProject(projectSFID)
	if err != nil {
		return nil, err
	}
	var externalProjectID string
	if project.Parent == "" || project.Parent == utils.TheLinuxFoundation {
		externalProjectID = projectSFID
	} else {
		externalProjectID = project.Parent
	}
	resp, err := s.repo.AddGithubOrganization(ctx, externalProjectID, projectSFID, &in)
	if err != nil {
		return nil, err
	}
	return v2GithubOrgnizationModel(resp)
}

func (s service) DeleteGithubOrganization(ctx context.Context, projectSFID string, githubOrgName string) error {
	psc := v2ProjectService.GetClient()
	_, projecterr := psc.GetProject(projectSFID)
	if projecterr != nil {
		return projecterr
	}
	err := s.ghRepository.DisableRepositoriesOfGithubOrganization(ctx, projectSFID, githubOrgName)
	if err != nil {
		return err
	}
	return s.repo.DeleteGithubOrganization(ctx, "", projectSFID, githubOrgName)
}

func (s service) UpdateGithubOrganization(ctx context.Context, projectSFID string, organizationName string, autoEnabled bool, branchProtectionEnabled bool) error {
	return s.repo.UpdateGithubOrganization(ctx, projectSFID, organizationName, autoEnabled, branchProtectionEnabled)
}

// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

import * as pulumi from '@pulumi/pulumi';
import * as aws from '@pulumi/aws';
import {PrivateAcl, PublicReadAcl} from '@pulumi/aws/s3';

const accountID = aws.getCallerIdentity().accountId;
const stackName = pulumi.getStack();
const stage = pulumi.getStack();
const region = aws.getRegion().name;
// Enable point in time recover on the production databases
const pointInTimeRecoveryEnabled = (stage == 'prod');

// Set to true if you want pulumi to import an existing resources in AWS into
// the current stack. This is typically done the first time only and once it is
// part of the stack, we set this value to false since it would then be under
// control of pulumi.
const importResources = false;
const defaultReadCapacity = 10;
const defaultWriteCapacity = 1;

console.log('Pulumi project name is  : ' + pulumi.getProject());
console.log('Pulumi stack name is    : ' + stackName);
console.log('Account ID is           : ' + accountID);
console.log('STAGE is                : ' + stage);
console.log('Region is               : ' + aws.getRegion().name);
console.log('Point In Time Recovery  : ' + pointInTimeRecoveryEnabled);
console.log('Default read capacity   : ' + defaultReadCapacity);
console.log('Default write capacity  : ' + defaultWriteCapacity);

const defaultTags = {
  Product: 'EasyCLA',
  ManagedBy: 'Pulumi',
  PulumiStack: pulumi.getStack(),
  STAGE: stage,
  ServiceType: 'Product',
  Service: 'Database',
  ServiceRole: 'Backend',
  Owner: 'David Deal',
};

const cfTags = {
  STAGE: stage,
};

const logoBucket = buildLogoBucket(importResources);
const logoBucketPolicy = buildLogoBucketPolicy();
const signatureFilesBucket = buildSignatureFilesBucket(importResources);
const projectsTable = buildProjectsTable(importResources);
const usersTable = buildUsersTable(importResources);
const companiesTable = buildCompaniesTable(importResources);
const signaturesTable = buildSignaturesTable(importResources);
const repositoriesTable = buildRepositoriesTable(importResources);
const gitHubOrgsTable = buildGitHubOrgsTable(importResources);
const gerritInstancesTable = buildGerritInstancesTable(importResources);
const userPermissionsTable = buildUserPermissionsTable(importResources);
const companyInvitesTable = buildCompanyInvitesTable(importResources);
const claManagerRequestsTable = buildCLAManagerRequestsTable(importResources);
const storeTable = buildStoreTable(importResources);
const sessionStoreTable = buildSessionStoreTable(importResources);
const eventsTable = buildEventsTable(importResources);
const cclaWhitelistRequestsTable = buildCclaWhitelistRequestsTable(importResources);
const metricsTable = buildMetricsTable(importResources);
const projectsClaGroupsTable = buildProjectsClaGroupsTable(importResources);

/**
 * Build the Logo S3 Bucket.
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildLogoBucket(importResources: boolean): aws.s3.Bucket {
  return new aws.s3.Bucket(
    'cla-project-logo-' + stage,
    {
      bucket: 'cla-project-logo-' + stage,
      acl: PublicReadAcl,
      region: region,
      corsRules: [
        {
          allowedHeaders: ['*'],
          allowedMethods: ['GET'],
          allowedOrigins: ['*'],
          //  'https://project.' + stage + '.lfcla.com',
          //  'https://project.lfcla.com',
          //  'https://corporate.' + stage + '.lfcla.com',
          //  'https://corporate.lfcla.com',
          //  'https://contributor.' + stage + '.lfcla.com',
          //  'https://contributor.lfcla.com',
          //],
          exposeHeaders: ['ETag'],
          maxAgeSeconds: 3000,
        },
      ],
      tags: defaultTags,
    },
    importResources ? { import: 'cla-project-logo-' + stage } : {},
  );
}

/**
 * Build the Logo S3 Bucket Policy.
 */
function buildLogoBucketPolicy(): aws.s3.BucketPolicy {
  return new aws.s3.BucketPolicy('cla-project-logo-' + stage + '/bucket-policy', {
    bucket: logoBucket.bucket,
    // Public read
    policy: logoBucket.bucket.apply(publicReadPolicyForBucket),
  });
}

function publicReadPolicyForBucket(bucketName: string) {
  return JSON.stringify({
    Version: '2012-10-17',
    Statement: [
      {
        Effect: 'Allow',
        Principal: '*',
        Action: ['s3:GetObject'],
        Resource: [`arn:aws:s3:::${bucketName}/*`],
      },
    ],
  });
}

/**
 * Build the Signature Files S3 Bucket
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildSignatureFilesBucket(importResources: boolean): aws.s3.Bucket {
  return new aws.s3.Bucket(
    'cla-signature-files-' + stage,
    {
      bucket: 'cla-signature-files-' + stage,
      acl: PrivateAcl,
      region: region,
      corsRules: [
        {
          allowedHeaders: ['*'],
          allowedMethods: ['GET'],
          allowedOrigins: ['*'],
          exposeHeaders: ['ETag'],
          maxAgeSeconds: 3000,
        },
      ],
      tags: defaultTags,
    },
    importResources ? { import: 'cla-signature-files-' + stage } : {},
  );
}

/**
 * Build Projects Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildProjectsTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-projects',
    {
      name: 'cla-' + stage + '-projects',
      attributes: [
        { name: 'project_id', type: 'S' },
        { name: 'project_external_id', type: 'S' },
        { name: 'project_name', type: 'S' },
        { name: 'project_name_lower', type: 'S' },
        { name: 'foundation_sfid', type: 'S' },
      ],
      hashKey: 'project_id',
      billingMode: 'PAY_PER_REQUEST',
      streamEnabled: true,
      streamViewType: "NEW_AND_OLD_IMAGES",
      globalSecondaryIndexes: [
        {
          name: 'external-project-index',
          hashKey: 'project_external_id',
          projectionType: 'ALL',
        },
        {
          name: 'project-name-search-index',
          hashKey: 'project_name',
          projectionType: 'ALL',
        },
        {
          name: 'project-name-lower-search-index',
          hashKey: 'project_name_lower',
          projectionType: 'ALL',
        },
        {
          name: 'foundation-sfid-project-name-index',
          hashKey: 'foundation_sfid',
          rangeKey: 'project_name',
          projectionType: 'ALL',
        },
      ],
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-projects' } : {},
  );
}

/**
 * Users Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildUsersTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-users',
    {
      name: 'cla-' + stage + '-users',
      attributes: [
        { name: 'user_id', type: 'S' },
        { name: 'user_github_id', type: 'S' },
        { name: 'user_github_username', type: 'S' },
        { name: 'lf_username', type: 'S' },
        { name: 'lf_email', type: 'S' },
        { name: 'user_external_id', type: 'S' },
      ],
      hashKey: 'user_id',
      billingMode: 'PAY_PER_REQUEST',
      streamEnabled: true,
      streamViewType: "NEW_AND_OLD_IMAGES",
      globalSecondaryIndexes: [
        {
          name: 'github-username-index',
          hashKey: 'user_github_username',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity
        },
        {
          name: 'lf-username-index',
          hashKey: 'lf_username',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity
        },
        {
          name: 'lf-email-index',
          hashKey: 'lf_email',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity
        },
        {
          name: 'github-user-index',
          hashKey: 'user_github_id',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity,
        },
        {
          name: 'github-user-external-id-index',
          hashKey: 'user_external_id',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity,
        },
      ],
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-users' } : {},
  );
}

/**
 * Companies Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildCompaniesTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-companies',
    {
      name: 'cla-' + stage + '-companies',
      attributes: [
        { name: 'company_id', type: 'S' },
        { name: 'company_external_id', type: 'S' },
        { name: 'company_name', type: 'S' },
      ],
      hashKey: 'company_id',
      billingMode: 'PAY_PER_REQUEST',
      streamEnabled: true,
      streamViewType: "NEW_AND_OLD_IMAGES",
      globalSecondaryIndexes: [
        {
          name: 'external-company-index',
          hashKey: 'company_external_id',
          projectionType: 'ALL',
        },
        {
          name: 'company-name-index',
          hashKey: 'company_name',
          projectionType: 'ALL',
        },
      ],
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-companies' } : {},
  );
}

/**
 * Signatures Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildSignaturesTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-signatures',
    {
      name: 'cla-' + stage + '-signatures',
      attributes: [
        { name: 'signature_id', type: 'S' },
        { name: 'signature_type', type: 'S' },
        { name: 'signature_project_id', type: 'S' },
        { name: 'signature_project_external_id', type: 'S' },
        { name: 'signature_reference_id', type: 'S' },
        { name: 'signature_user_ccla_company_id', type: 'S' },
        { name: 'signature_company_signatory_id', type: 'S' },
        { name: 'signature_reference_name_lower', type: 'S' },
        { name: 'signature_company_initial_manager_id', type: 'S' },
        { name: 'sigtype_signed_approved_id', type: 'S' },
      ],
      hashKey: 'signature_id',
      billingMode: 'PAY_PER_REQUEST',
      streamEnabled: true,
      streamViewType: "NEW_AND_OLD_IMAGES",
      globalSecondaryIndexes: [
        {
          name: 'project-signature-index',
          hashKey: 'signature_project_id',
          projectionType: 'ALL',
        },
        {
          name: 'reference-signature-index',
          hashKey: 'signature_reference_id',
          projectionType: 'ALL',
        },
        {
          name: 'signature-user-ccla-company-index',
          hashKey: 'signature_user_ccla_company_id',
          rangeKey: 'signature_project_id',
          projectionType: 'ALL',
        },
        {
          name: 'project-signature-external-id-index',
          hashKey: 'signature_project_external_id',
          projectionType: 'ALL',
        },
        {
          name: 'signature-company-signatory-index',
          hashKey: 'signature_company_signatory_id',
          projectionType: 'ALL',
        },
        {
          name: 'reference-signature-search-index',
          hashKey: 'signature_project_id',
          rangeKey: 'signature_reference_name_lower',
          projectionType: 'ALL',
        },
        {
          name: 'signature-project-id-type-index',
          hashKey: 'signature_project_id',
          rangeKey: 'signature_type',
          projectionType: 'ALL',
        },
        {
          name: 'signature-company-initial-manager-index',
          hashKey: 'signature_company_initial_manager_id',
          projectionType: 'ALL',
        },
        {
          name: 'signature-project-id-sigtype-signed-approved-id-index',
          hashKey: 'signature_project_id',
          rangeKey: 'sigtype_signed_approved_id',
          projectionType: 'ALL',
        },
      ],
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-signatures' } : {},
  );
}

/**
 * Repositories Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildRepositoriesTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-repositories',
    {
      name: 'cla-' + stage + '-repositories',
      attributes: [
        { name: 'repository_id', type: 'S' },
        { name: 'repository_external_id', type: 'S' },
        { name: 'repository_project_id', type: 'S' },
        { name: 'repository_sfdc_id', type: 'S' },
      ],
      hashKey: 'repository_id',
      billingMode: 'PROVISIONED',
      streamEnabled: true,
      streamViewType: "NEW_AND_OLD_IMAGES",
      readCapacity: defaultReadCapacity,
      writeCapacity: defaultWriteCapacity,
      globalSecondaryIndexes: [
        {
          name: 'sfdc-repository-index',
          hashKey: 'repository_sfdc_id',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity,
        },
        {
          name: 'project-repository-index',
          hashKey: 'repository_project_id',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity,
        },
        {
          name: 'external-repository-index',
          hashKey: 'repository_external_id',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity,
        },
      ],
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-repositories' } : {},
  );
}

/**
 * GitHub Organizations Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildGitHubOrgsTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-github-orgs',
    {
      name: 'cla-' + stage + '-github-orgs',
      attributes: [
        { name: 'organization_name', type: 'S' },
        { name: 'organization_sfid', type: 'S' },
      ],
      hashKey: 'organization_name',
      billingMode: 'PROVISIONED',
      streamEnabled: true,
      streamViewType: "NEW_AND_OLD_IMAGES",
      readCapacity: defaultReadCapacity,
      writeCapacity: defaultWriteCapacity,
      globalSecondaryIndexes: [
        {
          name: 'github-org-sfid-index',
          hashKey: 'organization_sfid',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity,
        },
      ],
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-github-orgs' } : {},
  );
}

/**
 * Gerrit Instances Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildGerritInstancesTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-gerrit-instances',
    {
      name: 'cla-' + stage + '-gerrit-instances',
      attributes: [{ name: 'gerrit_id', type: 'S' }],
      hashKey: 'gerrit_id',
      billingMode: 'PROVISIONED',
      streamEnabled: true,
      streamViewType: "NEW_AND_OLD_IMAGES",
      readCapacity: defaultReadCapacity,
      writeCapacity: defaultWriteCapacity,
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-gerrit-instances' } : {},
  );
}

/**
 * User Permissions Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildUserPermissionsTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-user-permissions',
    {
      name: 'cla-' + stage + '-user-permissions',
      attributes: [{ name: 'username', type: 'S' }],
      hashKey: 'username',
      readCapacity: defaultReadCapacity,
      writeCapacity: defaultWriteCapacity,
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-user-permissions' } : {},
  );
}

/**
 * Company Invites Table
 */
function buildCompanyInvitesTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-company-invites',
    {
      name: 'cla-' + stage + '-company-invites',
      attributes: [
        { name: 'company_invite_id', type: 'S' },
        { name: 'requested_company_id', type: 'S' },
      ],
      hashKey: 'company_invite_id',
      readCapacity: defaultReadCapacity,
      writeCapacity: defaultWriteCapacity,
      globalSecondaryIndexes: [
        {
          name: 'requested-company-index',
          hashKey: 'requested_company_id',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity,
        },
      ],
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-company-invites' } : {},
  );
}

/**
 * CLA Manager Requests Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildCLAManagerRequestsTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-cla-manager-requests',
    {
      name: 'cla-' + stage + '-cla-manager-requests',
      attributes: [
        { name: 'request_id', type: 'S' },
        { name: 'company_id', type: 'S' },
        { name: 'company_external_id', type: 'S' },
        { name: 'project_id', type: 'S' },
        { name: 'project_external_id', type: 'S' },
      ],
      hashKey: 'request_id',
      readCapacity: defaultReadCapacity,
      writeCapacity: defaultWriteCapacity,
      globalSecondaryIndexes: [
        {
          name: 'cla-manager-requests-company-project-index',
          hashKey: 'company_id',
          rangeKey: 'project_id',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity,
        },
        {
          name: 'cla-manager-requests-external-company-project-index',
          hashKey: 'company_external_id',
          rangeKey: 'project_external_id',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity,
        },
      ],
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-cla-manager-requests' } : {},
  );
}

/**
 * Store Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildStoreTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-store',
    {
      name: 'cla-' + stage + '-store',
      attributes: [{ name: 'key', type: 'S' }],
      hashKey: 'key',
      readCapacity: defaultReadCapacity,
      writeCapacity: defaultWriteCapacity,
      ttl: {
        attributeName: 'expire',
        enabled: true,
      },
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-store' } : {},
  );
}

/**
 * Session Store Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildSessionStoreTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-session-store',
    {
      name: 'cla-' + stage + '-session-store',
      attributes: [{ name: 'id', type: 'S' }],
      hashKey: 'id',
      readCapacity: defaultReadCapacity,
      writeCapacity: defaultWriteCapacity,
      ttl: {
        attributeName: 'expire',
        enabled: true,
      },
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-session-store' } : {},
  );
}

/**
 * Events Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildEventsTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-events',
    {
      name: 'cla-' + stage + '-events',
      attributes: [
        { name: 'event_id', type: 'S' },
        { name: 'event_type', type: 'S' },
        { name: 'event_user_id', type: 'S' },
        { name: 'event_project_id', type: 'S' },
        { name: 'event_date_and_contains_pii', type: 'B' },
        { name: 'company_id_external_project_id', type: 'S' },
        { name: 'event_time_epoch', type: 'N' },
        { name: 'company_sfid_foundation_sfid', type: 'S' },
        { name: 'company_sfid_project_id', type: 'S' },
        { name: 'event_foundation_sfid', type: 'S' },
      ],
      hashKey: 'event_id',
      readCapacity: defaultReadCapacity,
      writeCapacity: defaultWriteCapacity,
      streamEnabled: true,
      streamViewType: "NEW_AND_OLD_IMAGES",
      globalSecondaryIndexes: [
        { name: 'event-type-index', hashKey: 'event_type', projectionType: 'ALL', readCapacity: 1, writeCapacity: 1 },
        {
          name: 'event-user-id-index',
          hashKey: 'event_user_id',
          projectionType: 'ALL',
          readCapacity: 1,
          writeCapacity: 1
        },
        {
          name: 'event-project-id-event-time-epoch-index',
          hashKey: 'event_project_id',
          rangeKey: 'event_time_epoch',
          projectionType: 'ALL',
          readCapacity: 1,
          writeCapacity: 1
        },
        {
          name: 'company-sfid-foundation-sfid-event-time-epoch-index',
          hashKey: 'company_sfid_foundation_sfid',
          rangeKey: 'event_time_epoch',
          projectionType: 'ALL',
          readCapacity: 1,
          writeCapacity: 1
        },
        {
          name: 'company-sfid-project-id-event-time-epoch-index',
          hashKey: 'company_sfid_project_id',
          rangeKey: 'event_time_epoch',
          projectionType: 'ALL',
          readCapacity: 1,
          writeCapacity: 1
        },
        {
          name: 'event-foundation-sfid-event-time-epoch-index',
          hashKey: 'event_foundation_sfid',
          rangeKey: 'event_time_epoch',
          projectionType: 'ALL',
          readCapacity: 1,
          writeCapacity: 1
        },
        {
          name: 'event-date-and-contains-pii-event-time-epoch-index',
          hashKey: 'event_date_and_contains_pii',
          rangeKey: 'event_time_epoch',
          projectionType: 'ALL',
          readCapacity: 1,
          writeCapacity: 1
        },
        {
          name: 'company-id-external-project-id-event-epoch-time-index',
          hashKey: 'company_id_external_project_id',
          rangeKey: 'event_time_epoch',
          projectionType: 'ALL',
          readCapacity: 1,
          writeCapacity: 1
        },
      ],
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-events' } : {},
  );
}

/**
 * CclaWhitelistRequests Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildCclaWhitelistRequestsTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-ccla-whitelist-requests',
    {
      name: 'cla-' + stage + '-ccla-whitelist-requests',
      attributes: [
        { name: 'request_id', type: 'S' },
        { name: 'company_id', type: 'S' },
        { name: 'project_id', type: 'S' },
      ],
      hashKey: 'request_id',
      readCapacity: defaultReadCapacity,
      writeCapacity: defaultWriteCapacity,
      globalSecondaryIndexes: [
        {
          name: 'company-id-project-id-index',
          hashKey: 'company_id',
          rangeKey: "project_id",
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: defaultWriteCapacity
        },
      ],
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-ccla-whitelist-requests' } : {},
  );
}

/**
 * Metrics Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildMetricsTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + '-metrics',
    {
      name: 'cla-' + stage + '-metrics',
      attributes: [
        { name: 'id', type: 'S' },
        { name: 'salesforce_id', type: 'S' },
        { name: 'metric_type', type: 'S' },
      ],
      hashKey: 'metric_type',
      rangeKey: "id",
      readCapacity: defaultReadCapacity,
      writeCapacity: 5,
      globalSecondaryIndexes: [
        {
          name: 'metric-type-salesforce-id-index',
          hashKey: 'metric_type',
          rangeKey: "salesforce_id",
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: 5
        },
      ],
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-metrics' } : {},
  );
}

/**
 * ProjectsClaGroups Table
 *
 * @param importResources flag to indicate if we should import the resources
 * into our stack from the provider (rather than creating it for the first
 * time).
 */
function buildProjectsClaGroupsTable(importResources: boolean): aws.dynamodb.Table {
  return new aws.dynamodb.Table(
    'cla-' + stage + 'projects-cla-groups',
    {
      name: 'cla-' + stage + '-projects-cla-groups',
      attributes: [
        { name: 'project_sfid', type: 'S' },
        { name: 'cla_group_id', type: 'S' },
        { name: 'foundation_sfid', type: 'S' },
      ],
      hashKey: 'project_sfid',
      readCapacity: defaultReadCapacity,
      writeCapacity: 1,
      streamEnabled: true,
      streamViewType: "NEW_AND_OLD_IMAGES",
      globalSecondaryIndexes: [
        {
          name: 'cla-group-id-index',
          hashKey: 'cla_group_id',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: 1
        },
        {
          name: 'foundation-sfid-index',
          hashKey: 'foundation_sfid',
          projectionType: 'ALL',
          readCapacity: defaultReadCapacity,
          writeCapacity: 1
        },
      ],
      pointInTimeRecovery: {
        enabled: pointInTimeRecoveryEnabled,
      },
      tags: defaultTags,
    },
    importResources ? { import: 'cla-' + stage + '-projects-cla-groups' } : {},
  );
}

// DynamoDB trigger events handler functions
const dynamoDBSignaturesEventLambdaName = "cla-backend-" + stage + "-dynamo-signatures-events-lambda";
const dynamoDBSignaturesEventLambdaArn = "arn:aws:lambda:" + aws.getRegion().name + ":" + accountID + ":function:" + dynamoDBSignaturesEventLambdaName;
signaturesTable.onEvent("signatureStreamEvents",
  aws.lambda.Function.get(dynamoDBSignaturesEventLambdaName, dynamoDBSignaturesEventLambdaArn),
  { startingPosition: "LATEST" });

const dynamoDBEventsEventLambdaName = "cla-backend-" + stage + "-dynamo-events-events-lambda";
const dynamoDBEventsEventLambdaArn = "arn:aws:lambda:" + aws.getRegion().name + ":" + accountID + ":function:" + dynamoDBEventsEventLambdaName;
eventsTable.onEvent("eventStreamEvents",
  aws.lambda.Function.get(dynamoDBEventsEventLambdaName, dynamoDBEventsEventLambdaArn),
  { startingPosition: "LATEST" });

const dynamoDBProjectsCLAGroupsEventLambdaName = "cla-backend-" + stage + "-dynamo-projects-cla-groups-events-lambda";
const dynamoDBProjectsCLAGroupsEventLambdaArn = "arn:aws:lambda:" + aws.getRegion().name + ":" + accountID + ":function:" + dynamoDBProjectsCLAGroupsEventLambdaName;
projectsClaGroupsTable.onEvent("projectsCLAGroupsStreamEvents",
  aws.lambda.Function.get(dynamoDBProjectsCLAGroupsEventLambdaName, dynamoDBProjectsCLAGroupsEventLambdaArn),
  { startingPosition: "LATEST" });

// Export the name of the bucket
export const logoBucketARN = logoBucket.arn;
export const logoBucketName = logoBucket.bucket;
export const logoBucketPolicyOutput = logoBucketPolicy.policy;
export const signatureFilesBucketARN = signatureFilesBucket.arn;
export const signatureFilesBucketName = signatureFilesBucket.bucket;
export const projectsTableName = projectsTable.name;
export const projectsTableARN = projectsTable.arn;
export const companiesTableName = companiesTable.name;
export const companiesTableARN = companiesTable.arn;
export const usersTableName = usersTable.name;
export const usersTableARN = usersTable.arn;
export const signaturesTableName = signaturesTable.name;
export const signaturesTableARN = signaturesTable.arn;
export const repositoriesTableName = repositoriesTable.name;
export const repositoriesTableARN = repositoriesTable.arn;
export const gitHubOrgsTableName = gitHubOrgsTable.name;
export const gitHubOrgTableARN = gitHubOrgsTable.arn;
export const gerritInstancesTableName = gerritInstancesTable.name;
export const gerritInstancesTableARN = gerritInstancesTable.arn;
export const userPermissionsTableName = userPermissionsTable.name;
export const userPermissionsTableARN = userPermissionsTable.arn;
export const companyInvitesTableName = companyInvitesTable.name;
export const companyInvitesTableARN = companyInvitesTable.arn;
export const claManagerRequestsTableName = claManagerRequestsTable.name;
export const claManagerRequestsTableARN = claManagerRequestsTable.arn;
export const storeTableName = storeTable.name;
export const storeTableARN = storeTable.arn;
export const sessionStoreTableName = sessionStoreTable.name;
export const sessionStoreTableARN = sessionStoreTable.arn;
export const eventsTableName = eventsTable.name;
export const eventsTableARN = eventsTable.arn;
export const cclaWhitelistRequestsTableName = cclaWhitelistRequestsTable.name;
export const cclaWhitelistRequestsTableARN = cclaWhitelistRequestsTable.arn;
export const metricsTableName = metricsTable.name;
export const metricsTableARN = metricsTable.arn;
export const projectsClaGroupsTableName = projectsClaGroupsTable.name
export const projectsClaGroupsTableARN = projectsClaGroupsTable.arn

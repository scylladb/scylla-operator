# Grafana authentication with various OAuth providers 

## Summary

This proposal focuses on a new authentication method for Grafana, which allows users to [authenticate with various OAuth providers](https://grafana.com/docs/grafana/latest/setup-grafana/configure-security/configure-authentication) such as:
- Generic OAuth authentication
- Okta

It also introduces very basic authz solution via Grafana roles, which allows users to be assigned to a role based on their group membership in the OAuth provider. This is achieved with JMESPath queries and `role_attribute_path` [configuration option](https://grafana.com/docs/grafana/latest/setup-grafana/configure-security/configure-authentication/generic-oauth/#role-mapping).

## Motivation

<!-- This section is for explicitly listing the motivation, goals and non-goals of
this proposal. Describe why the change is important and the benefits to users. -->

The motivation for this proposal is to allow users to authenticate with various OAuth providers, which is a common use case for many organizations. Not only does it allow users to authenticate with their existing credentials, but it also allows organizations to manage access to Grafana via their existing identity management system.

### Goals

<!-- List the specific goals of the proposal. How will we know that this has succeeded? -->

- Extend `ScyllaDBMonitoring` CRD to expose more configuration options for Grafana authentication options
- Grafana authentication options must allow to configure:
  - Generic OAuth authentication
  - Okta authentication
  - Multiple OAuth providers at the same time(this is supported by Grafana)
  - Role mapping via JMESPath queries for each OAuth provider


### Non-Goals

--

## Proposal

<!-- This is where we get down to the specifics of what the enhancement actually is.
This should have enough detail that reviewers can understand exactly what
you're proposing, but should not include things like API designs or
implementation. What is the desired outcome and how do we measure success?.
The "Design Details" section below is for the real nitty-gritty. -->

This proposal focuses on a new authentication method for Grafana, which allows users to [authenticate with various OAuth providers](https://grafana.com/docs/grafana/latest/setup-grafana/configure-security/configure-authentication) such as:
- Generic OAuth authentication
- Okta

It also introduces very basic authz solution via Grafana roles, which allows users to be assigned to a role based on their group membership in the OAuth provider. This is achieved with JMESPath queries and `role_attribute_path` [configuration option](https://grafana.com/docs/grafana/latest/setup-grafana/configure-security/configure-authentication/generic-oauth/#role-mapping).

The desired outcome is to allow users to configure OAuth provider of their choise as an authentication method for Grafana and be properly mapped to Grafana roles based on their group membership in the OAuth provider using JMESPath queries.

JSMEPath queries should be validated for syntacticall correctness, but any other validation is out of scope of this proposal.


### User Stories

--

### Notes/Constraints/Caveats [Optional]

--

### Risks and Mitigations

- Increased complexity of the `ScyllaDBMonitoring` CRD and Grafana configuration.
- Increased chance to misconfigure Grafana authentication options.

## Design Details

Add `oauth` section to `grafana/authentication` configuration located in `ScyllaDBMonitoring` that accepts various OAuth providers configuration options.

```yaml
authentication:
    insecureEnableAnonymousAccess: true
    oauth:
        providers:
        -   name: generic
            enabled: true
            allowSignUp: true
            autoLogin: false
            clientId: clientId
            clientSecret:
                valueFrom:
                    secretKeyRef:
                        name: 'generic-client-secret'
                        key: 'clientSecret'
            scopes: openid email name
            authUrl: https://<onelogin domain>.onelogin.com/oidc/2/auth
            tokenUrl: https://<onelogin domain>.onelogin.com/oidc/2/token
            apiUrl: https://<onelogin domain>.onelogin.com/oidc/2/me
            teamIds:
            allowedOrganizations:
            roleAttributePath: #JMESPath
        -   name: okta
            icon: okta
            enabled: true
            allowSignUp: true
            clientId: clientId
            clientSecret:
                valueFrom:
                    secretKeyRef:
                        name: 'okta-client-secret'
                        key: 'clientSecret'
            scopes: openid profile email groups
            authUrl: https://<tenant-id>.okta.com/oauth2/v1/authorize
            tokenUrl: https://<tenant-id>.okta.com/oauth2/v1/token
            apiUrl: https://<tenant-id>.okta.com/oauth2/v1/userinfo
            allowedDomains:
            allowedGroups: 
            usePkce: true
            roleAttributePath: #JMESPath
```

Properties for each OAuth provider are described in [Grafana documentation](https://grafana.com/docs/grafana/latest/setup-grafana/configure-security/configure-authentication/). This probably needs to follow the same structure as the `grafana.ini` file unless there's a reason to come up with a new API for this purpose.

### Test Plan

I believe this feature requires to be covered by e2e tests, which will test the following scenarios:
- Generic OAuth authentication
- Okta authentication
- Role mapping via JMESPath queries for each OAuth provider(optional, can be covered by manual testing since I predict it might be hard to test, perhaps some kind of validation for the `role_attribute_path` configuration option would be helpful)
- Multiple OAuth providers configured at the same time

### Upgrade / Downgrade Strategy

--

### Version Skew Strategy

--

## Implementation History

- 2023-06-21: Initial proposal

## Drawbacks

Should this proposal be implemented it adds additional responsibility for the Operator team to maintain this API follwing Grafana changes.

## Alternatives

Grafana can be hosted outside of the ScyllaDB Operator, which allows users to configure Grafana authentication options manually. However, this approach moves the responsibility of managing Grafana deployment to the user, which is not ideal since Scylla Operator supports Scylla Monitoring Stack out of the box.

## Infrastructure Needed [optional]

OAuth test environment is needed to test this feature.

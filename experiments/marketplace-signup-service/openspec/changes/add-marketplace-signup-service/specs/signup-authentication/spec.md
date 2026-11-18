# Spec Delta

## Purpose

Server-side verification of the customer's Google identity: validating the Google ID token presented at sign-up, restricting accepted identities to Red Hat's hosted domain, and deriving the normalized email used as the RoleBinding subject.

## ADDED Requirements

### Requirement: Server-side Google ID token validation

THE SYSTEM SHALL validate every presented Google ID token server-side before trusting any identity claim: verify the signature against Google's published JWKS, require the `aud` claim to equal the configured OAuth client ID, require a non-expired token, and require `email_verified` to be true.

#### Scenario: Valid token accepted

- **WHEN** the backend receives a sign-up request whose ID token has a valid Google signature, `aud` equal to the configured client ID, unexpired, and `email_verified == true`
- **THEN** the backend extracts the `email` claim as the caller's identity

#### Scenario: Token with wrong audience

- **WHEN** the ID token's `aud` does not equal the configured OAuth client ID
- **THEN** the request is rejected with an authentication error and no further processing occurs

#### Scenario: Unverified or expired token

- **WHEN** the ID token has `email_verified` false or missing, or is expired, or has an invalid signature
- **THEN** the request is rejected with an authentication error and no further processing occurs

### Requirement: Restricted to Red Hat identities

THE SYSTEM SHALL accept only identities from Red Hat's Google Workspace domain: the Google-issued `hd` claim SHALL equal a configured allowed domain (initially `redhat.com`), and the normalized email's domain SHALL agree with it. The allowlist SHALL be configuration; enforcement SHALL be server-side only, with any client-sent `hd` hint treated as presentation rather than control.

#### Scenario: Red Hat Workspace identity accepted

- **WHEN** a user signs in with a Red Hat Google Workspace account (`hd == redhat.com`, email ending in `@redhat.com`)
- **THEN** authentication succeeds and sign-up may proceed

#### Scenario: Consumer Google account rejected

- **WHEN** a user signs in with a personal Google account (no `hd` claim, e.g. `@gmail.com`)
- **THEN** the request is rejected with an authorization error and no RoleBinding is created

#### Scenario: External hosted domain rejected

- **WHEN** the ID token's `hd` claim names a Workspace domain not in the allowlist, or the email domain does not match `hd`
- **THEN** the request is rejected with an authorization error before any project lookup

### Requirement: Email normalization matching the authorization model

THE SYSTEM SHALL normalize the verified email identically to the platform's RoleBinding subject normalization: NFC Unicode normalization, lowercase the domain part, and preserve the local-part case.

#### Scenario: Normalized subject used for binding

- **WHEN** a sign-up succeeds for the email `Alice@Example.COM`
- **THEN** the RoleBinding subject is created as `Alice@example.com`

### Requirement: No trust in client-side identity claims

THE SYSTEM SHALL NOT accept any identity claim (email, project association) from the client other than the cryptographically validated ID token.

#### Scenario: Spoofed identity headers ignored

- **WHEN** a request arrives with identity-asserting headers or body fields set by the client other than the ID token
- **THEN** those assertions are ignored and identity is derived only from token validation

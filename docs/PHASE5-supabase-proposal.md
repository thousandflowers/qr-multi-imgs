# Phase 5, Supabase: a proposal, not a build

**Status: proposal only. Nothing here has been run.** No migration applied, no
function deployed, no project created, no key issued or read. Every SQL
statement and every function body below is text in a document. If this is ever
built, it starts by someone reading this and disagreeing with parts of it.

## What this is for, and the decision it sits under

[Phase 4](ROADMAP-webui-assisted-decode.md#phase-4-optional-contribution) asks
whether a user who hits a code the decoder cannot read should be able to send
that crop in, so the failure becomes a labelled sample instead of a shrug.

**Phase 4 is currently decided as not being built.** Contribution is out of
scope, there is no upload endpoint, and the hosted page's
`connect-src 'self' http://127.0.0.1:* http://localhost:*` stays as it is so
that the README's claim keeps meaning what it says.

This document exists because "we would need a backend" is not an argument
until the backend has been drawn. What follows is the smallest hosted
component that would satisfy Phase 4's constraints, so that the cost of
reversing that decision is a number rather than a feeling. It is also the
record of one design that was considered and rejected outright, which is the
more useful half.

Read it as: *if this is built, this is the shape it has to be.*

## The two constraints that decide everything else

**One.** The page cannot be given a credential. Anything shipped to a browser
is public, and a public write credential is a public write endpoint with no
validation in front of it. This kills the obvious Supabase design, see
[the rejected alternative](#the-rejected-alternative-direct-client-writes-with-the-anon-key).

**Two.** The privacy claim is enforced by a CSP, and adding an origin to
`connect-src` weakens it for every visitor, including the ones who will never
contribute. Phase 4 lists three options and prefers the second: contribution
happens only in the local app (`--serve`), where a binary the user installed
does the upload, and the hosted page is untouched. **This proposal assumes
that option.** The uploader is the Go binary on the user's machine, not the
page. That changes the threat model in one useful way (the client is a build
we ship, not a page anyone can edit) and in one unhelpful way (it is still a
client on someone else's computer, so it is still not trusted).

## Shape

```
qr-multi-imgs (local binary, user ran it deliberately)
    |
    |  POST /functions/v1/contribute        multipart: crop + consent fields
    v
Edge Function "contribute"  ---- service role ----> Postgres  (contributions)
    |                                          \-> Storage    (private bucket)
    |
    +-- returns { id, deletion_token } once, and never again

qr-multi-imgs delete --token=...
    |
    |  POST /functions/v1/contribute-delete  { id, token }
    v
Edge Function "contribute-delete" -- verify hash --> soft delete + object removal
```

Three things are true of this diagram and each is load-bearing:

- **The database is never reachable from a client.** Not with the anon key,
  not with any key. Only the two functions touch it, with the service role,
  which never leaves the server.
- **The Edge Function is the only validator.** RLS cannot look at bytes.
- **The token is returned once.** The server keeps a hash and cannot recover
  it, which is what makes deletion possible without collecting identity.

## SQL schema

```sql
-- One row per contributed crop. There is deliberately no user, no email, no
-- device id and no IP column: the deletion token is the only thing that ties
-- a person to a row, and it is held by them, not by us.
create table public.contributions (
  id              uuid primary key default gen_random_uuid(),
  created_at      timestamptz not null default now(),

  -- Where the bytes live in the private bucket. Never a public URL.
  object_path     text        not null unique,

  -- What was accepted, recorded as accepted rather than as claimed: these are
  -- the values the function measured after decoding the image itself, not the
  -- ones the client sent in headers.
  mime            text        not null check (mime in ('image/png','image/jpeg','image/webp')),
  byte_size       integer     not null check (byte_size > 0 and byte_size <= 524288),
  width           integer     not null check (width  between 16 and 4096),
  height          integer     not null check (height between 16 and 4096),
  sha256          bytea       not null,

  -- The consent that was on screen when this was sent. Not a boolean: the
  -- purpose stated at collection is the ceiling on use, so the exact wording
  -- has to be recoverable years later. A row whose consent version is not in
  -- consent_versions is unusable, by construction.
  consent_version text        not null references public.consent_versions(version),
  ui_language     text        not null check (ui_language ~ '^[a-z]{2}$'),

  -- Deletion is a tombstone, not a DELETE: the row records that a deletion
  -- happened, without keeping anything about who asked. The object is really
  -- gone; this says so.
  deleted_at      timestamptz,

  -- Ground truth arrives by hand or not at all. A contributed crop is a crop
  -- the decoder failed on, so nobody knows what it says, possibly not even
  -- the contributor.
  labelled_at     timestamptz,
  label           text,

  constraint label_needs_a_time check ((label is null) = (labelled_at is null))
);

-- The deletion credential, hashed, in its own table so that a mistaken
-- "select *" on contributions cannot leak it and so it can be dropped
-- independently of the crop's metadata.
--
-- HMAC-SHA256 with a server-side pepper, not a bare hash: the token is 256
-- bits of random and is never reused, so the slow-hash argument for passwords
-- does not apply, but a pepper the database does not contain means a database
-- copy alone cannot verify tokens.
create table public.contribution_tokens (
  contribution_id uuid primary key references public.contributions(id) on delete cascade,
  token_hmac      bytea not null,
  created_at      timestamptz not null default now()
);

-- The wording, kept because the wording is the permission.
create table public.consent_versions (
  version    text primary key,           -- e.g. '2026-09-03.1'
  purpose_en text not null,              -- the English source text
  effective  timestamptz not null,
  retired    timestamptz
);

-- Rate limiting that keeps no IP. See the function notes: the key is an HMAC
-- of the address under a salt that is rotated hourly and never stored, so a
-- row here cannot be turned back into an address even by us, and after the
-- window it cannot be matched to a new request from the same address either.
create table public.rate_buckets (
  bucket_key  bytea       primary key,
  window_start timestamptz not null,
  count       integer     not null default 0
);

create index contributions_unlabelled
  on public.contributions (created_at)
  where deleted_at is null and labelled_at is null;
```

**On the absence of a retention column.** Phase 4 says not to state a
retention period unless it is enforced. There is no `expires_at` here because
there is no job to honour it, and a column that names a date nothing acts on
is a false statement with a schema around it. If retention is promised, it
arrives as a scheduled function and a column in the same change, or not at
all.

## Row Level Security

The policies below are short because the answer is "no" in every case. That is
the design, not an omission.

```sql
alter table public.contributions        enable row level security;
alter table public.contribution_tokens  enable row level security;
alter table public.consent_versions     enable row level security;
alter table public.rate_buckets         enable row level security;

-- force row level security so that even the table owner is subject to these
-- policies; the service role bypasses RLS by design and is the only intended
-- writer.
alter table public.contributions        force row level security;
alter table public.contribution_tokens  force row level security;
alter table public.rate_buckets         force row level security;

-- No policy is created for anon or authenticated on any table.
--
-- In Postgres, RLS with no permissive policy denies everything, so this is
-- not a table with cautious rules on it: it is a table no client role can
-- read, insert, update or delete a single row of. Writing it as an explicit
-- deny is clearer to the next reader than an empty policy list that looks
-- like an oversight.
create policy no_client_access_contributions on public.contributions
  for all to anon, authenticated using (false) with check (false);

create policy no_client_access_tokens on public.contribution_tokens
  for all to anon, authenticated using (false) with check (false);

create policy no_client_access_buckets on public.rate_buckets
  for all to anon, authenticated using (false) with check (false);

-- The consent wording is the one thing a client may read, because a client
-- has to be able to display it, and because publishing the terms you
-- collected under is the point of keeping them.
create policy consent_is_public on public.consent_versions
  for select to anon, authenticated using (retired is null);

revoke all on public.contributions       from anon, authenticated;
revoke all on public.contribution_tokens from anon, authenticated;
revoke all on public.rate_buckets        from anon, authenticated;
grant  select on public.consent_versions to  anon, authenticated;
```

**Storage.** One bucket, `crops`, created private, with no policy granting
anon or authenticated any operation. Objects are written and read by the
service role only. There is no signed URL flow for reading, because nothing
in the product reads a crop back; the only reader is a person labelling them
through an authenticated internal path that is out of this proposal's scope.

## Edge Function: `contribute`

```
POST /functions/v1/contribute
Content-Type: multipart/form-data

  crop            the image bytes, one part
  consent_version text, must match a current row in consent_versions
  ui_language     two letters, the language the consent was read in
  client          "qr-multi-imgs/<version>"

200 { "id": "<uuid>", "deletion_token": "<43 chars, base64url>" }
400 { "error": "<one of the codes below>" }
413 { "error": "too_large" }
429 { "error": "rate_limited", "retry_after": <seconds> }
```

### Validation, in the order it happens

Order matters: the cheap rejections come first so that an abusive client
cannot make the server do expensive work to say no.

1. **Method and content type.** Anything but `POST multipart/form-data` is
   rejected before the body is read.
2. **Declared length.** `Content-Length` above **512 KiB** is refused with 413
   without reading the body. A crop of one QR code is a few tens of KiB; half
   a megabyte is already generous and the ceiling exists to bound the work,
   not to be reached.
3. **Actual length.** The stream is counted as it is read and aborted at the
   same ceiling, because `Content-Length` is a claim.
4. **Magic bytes, not the declared type.** The first bytes are matched against
   PNG (`89 50 4E 47 0D 0A 1A 0A`), JPEG (`FF D8 FF`) and WebP
   (`RIFF....WEBP`). The multipart part's own `Content-Type` is not trusted
   for anything except being present. A mismatch is `bad_image`.
5. **It decodes, and its dimensions are sane.** The image is decoded server
   side. This is the only way to know that what arrived is an image rather
   than a file that starts like one, and it is where the width and height
   written to the row come from. Outside 16..4096 on either axis, or a decode
   failure, is `bad_image`.
6. **Re-encode before storing.** The stored object is the re-encoded output,
   not the received bytes. The client already re-encodes from canvas pixels so
   that EXIF dies by construction (Phase 4), and this does it again on the
   server for the same reason: a promise to ignore metadata is weaker than a
   pipeline that cannot carry it. `sha256` is of the stored bytes.
7. **Consent.** `consent_version` must name a row in `consent_versions` with
   `retired is null`. An unknown or retired version is `stale_consent`, and
   the client is expected to fetch the current wording and ask again. A crop
   collected under wording nobody can produce is a crop that cannot be used.
8. **Rate limit.** See below. Checked after validation so that a well-formed
   contribution is not counted against a budget it then fails to use.

Every rejection returns a code from a closed set:
`bad_method`, `bad_image`, `too_large`, `stale_consent`, `rate_limited`,
`server_error`. No error echoes any part of the input back.

### Rate limiting that persists no IP

This is the part of Phase 4 explicitly called "a genuine design problem", and
it does not have a clean answer, so here is the trade being made rather than a
claim that it is solved.

```
salt        = HMAC(pepper, floor(now / 1 hour))     // derived, never stored
bucket_key  = HMAC(salt, client_address)            // 32 bytes
```

- The address is used and discarded inside one request. It is never a column,
  never a log line, never part of the returned body.
- `bucket_key` is not reversible to an address without `pepper` and the hour,
  and the hour rolls forward: last hour's keys cannot be matched to this
  hour's requests even by us, even with the pepper.
- The budget is **5 accepted contributions per key per hour**, with a
  `retry_after` in the 429 so a client can behave.
- `rate_buckets` rows older than two hours are deleted by a scheduled job.
  This is the one piece of retention that must exist, because without it a
  table of unlinkable-but-plentiful keys accumulates for no purpose.

**What this does not do.** It is per-address, so it is defeated by many
addresses, and it is not a defence against a determined flood. It is a budget
that stops accidents and casual abuse. The alternatives Phase 4 lists are
proof-of-work (real cost to the honest contributor, and it is beaten by anyone
willing to spend CPU) and a per-session budget (defeated by clearing state).
None of the three is a security control, and pretending otherwise in a policy
page would be the actual failure. If a flood happens, the endpoint is turned
off. That is stated here so it is a plan rather than a surprise.

**The claim that must be checked before it is made.** "No IP logging" is a
property of the whole stack, not of this function. A hosted platform's own
edge and gateway logs typically record request addresses, and if the platform
cannot demonstrate that they are off, the honest README sentence is not "no IP
logging" but "our function keeps no address; the host may log one, see their
policy". **The claim does not get made until it has been verified in writing
for the specific project, in the specific region.** This is a blocking item,
not a detail.

## The deletion token

```
token       = base64url(random 32 bytes)             // 43 characters, shown once
token_hmac  = HMAC-SHA256(pepper, token)             // what the server keeps
```

**Issuing.** Generated by the function, never by the client, so its randomness
does not depend on a caller. Returned in the 200 body and written nowhere
else. It is not logged, not emailed, not recoverable. The client displays it
and tells the user, in the same breath, that losing it means the crop cannot
be deleted, because there is nothing else tying the crop to anyone. That
sentence is the price of collecting no identity, and it is stated at the
moment of collection, not in a policy page.

**Redeeming.**

```
POST /functions/v1/contribute-delete
{ "id": "<uuid>", "token": "<43 chars>" }

200 { "deleted": true }
404 { "error": "not_found" }
```

1. Look up `contribution_tokens` by `contribution_id`.
2. Compute `HMAC-SHA256(pepper, token)` and compare with **constant time**.
   A byte-by-byte comparison here leaks the prefix over enough attempts.
3. On match: delete the object from storage, delete the token row, and set
   `deleted_at` on the contribution. The crop is gone; the row remains as a
   tombstone that says a deletion happened and holds nothing about who asked.
4. On no match, and on an unknown id: **the same 404, after the same work.**
   A distinct "wrong token" reply would confirm that a given id exists, and
   an early return would say so in the timing.
5. The delete endpoint gets its own rate budget, keyed the same way, because
   it is the one endpoint that can be probed.

**What is deliberately not offered.** No "list my contributions", no "resend
my token", no recovery flow of any kind. Each would need something that
identifies a person, which is the thing this design exists to avoid holding.

## The rejected alternative: direct client writes with the anon key

The obvious Supabase design, and the one every tutorial shows: give the client
the anon key, `insert` into `contributions` from the client, upload to a
bucket with a storage policy, and write RLS `with check` rules to keep it
honest.

**Rejected.** Not as a preference, on four counts, any one of which is fatal.

**One: the anon key is public, so this is an open write endpoint.** It ships
in the client by definition. `insert` permitted to `anon` is `insert`
permitted to anyone who reads it out and writes their own loop, at whatever
rate they like, with whatever content they like. The RLS rule is not a gate on
who is asking, only on what a row looks like.

**Two: RLS cannot validate an image.** A `with check` constraint sees column
values. It cannot decode bytes, cannot confirm that an object is a PNG rather
than a file whose name ends in `.png`, cannot measure dimensions, and cannot
verify that the row's `byte_size` and `mime` describe the object the client
also uploaded. Every validation rule in the section above would become a
column the client fills in about itself. The bucket becomes a place to put
arbitrary files, with a metadata table that says whatever the uploader wanted
it to say.

**Three: the deletion token stops working.** For the client to insert the row,
it inserts the hash too, which means the client chooses the token and the
scheme's security rests on a caller we do not control. Worse, the hash sits in
a row an `insert`-capable role can also be tempted to `select`; keeping it
unreadable while letting the client write it needs exactly the split this
design already has, at which point the client is not writing directly.

**Four: rate limiting becomes impossible rather than merely hard.** There is
no server-side step to attach a budget to. PostgREST is the endpoint, and the
only thing in front of it is the platform's global limits, which are about
protecting the platform, not about how many crops one person may send. The
choice would be between no limit and a limit enforced by the client on itself.

**The general form**, worth writing down because it will come up again: RLS is
an authorisation mechanism, and this problem is a validation problem. RLS
answers "may this role touch this row", which is the right question when there
are accounts and a row belongs to someone. Here there are deliberately no
accounts, every writer is the same anonymous role, and the question is "are
these bytes acceptable", which no policy language can answer. **A public
credential cannot be made into a write-only, validated, rate-limited endpoint
by row policies.** It needs code in front of it, which is what the Edge
Function is.

## Cost, honestly

The function and the schema are perhaps three days. They are not the cost.

- **A hosted component the project does not have.** Today `server.go` is a
  loopback server on the user's own machine. This is the first thing that runs
  somewhere else, with an on-call implication, a bill, and a region that has
  to be chosen for legal reasons rather than latency.
- **A pepper to hold.** Its loss makes every outstanding deletion token
  unverifiable, which means every contributor loses the ability to delete. It
  cannot be rotated without invalidating live tokens.
- **The labelling backlog.** A contributed crop is a crop the decoder failed
  on, so it arrives with no ground truth. Unlabelled it is a picture, not a
  sample. The corpus grows only as fast as a person labels it, and the
  `contributions_unlabelled` index exists to make that queue visible rather
  than to pretend it is small.
- **A verified answer on platform logging** before the privacy sentence in
  the README can survive contact with this feature.

Against which the existing route costs nothing and gives more back: run
`corpusgen`, get a manifest with the undecoded images pre-labelled
`EXPECTED_FAIL`, correct them by hand, open a pull request. A labelled case
with a person attached to it.

**Which is why Phase 4 is not being built, and why this document ends without
a recommendation to build it.** It ends with a design, so that the decision
can be reversed on evidence rather than re-argued from scratch.

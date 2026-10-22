<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# Android (APK archive)

A hosted archive of Android apps. Unlike the package-manager formats it has no client protocol: files are uploaded and downloaded over HTTP, and the point of the format is what the server records about each file: who signed it, which key that was, where it came from and what was patched.

| | |
|---|---|
| Format id | `android` |
| Repository types | hosted only. See [hosted, proxy, group](../repositories/hosted-proxy-group.md) |
| Client URL | `HOST/repos/REPO/apps/<package>/...` (see [Downloading](#downloading)) |
| Management API | `HOST/api/v1/repositories/REPO/apks/...` (OpenAPI tag `Android`) |
| Parser | `pkg/apk`, standard library only; verified against `apksigner` and `aapt2` on real F-Droid APKs |

Repositories are private by default. Create one as an admin (the web UI does not list the format yet):

```sh
curl -X POST HOST/api/v1/repositories -H "Authorization: Bearer <TOKEN>" \
  -H 'Content-Type: application/json' -d '{"name":"phone","format":"android"}'
```

## What is recorded

Everything is read from the uploaded file; nothing is taken from the client except the hand-written fields below.

| Area | Content |
|---|---|
| Identity | package name, `versionCode` (64 bit), `versionName`, SHA-256, size |
| Manifest | min/target/compile SDK, `debuggable`, permissions, features, components with intent filters, decoded `AndroidManifest.xml` |
| Titles and icons | the app title for every language in `resources.arsc`, the launcher activity title when different, every icon by role (app, round, adaptive foreground/background/monochrome) and density, with size and format |
| Signatures | schemes present (v1 JAR, v2, v3, v3.1) and whether each verifies; per signer the certificate (subject, issuer, serial, validity, key type and size) and its SHA-256, SHA-1 and MD5 fingerprints; v3 rotation lineage |
| Flags | debug certificate, AOSP test keys, RSA below 2048 bits, expired certificate |
| Packaging | ABIs, native libraries with ELF details and 16 KiB alignment, zip alignment, uncompressed `resources.arsc`, duplicate entries, bytes before the zip |
| Build | framework (Flutter, React Native, Unity, ...), embedded Maven coordinates, Android Gradle plugin version |
| `content_digest` | digest of the file's contents without its signature: two files with the same value are the same build signed with different keys |

The SHA-256 fingerprint is the one `apksigner verify --print-certs`, F-Droid and AppVerifier print.

Parsing is tolerant: a malformed icon, title or signature becomes a warning stored with the version. Only a file that is not a zip, or has no readable manifest, is rejected.

## Uploading

`POST /api/v1/repositories/REPO/apks` (multipart, write access):

```sh
curl -X POST HOST/api/v1/repositories/REPO/apks -H "Authorization: Bearer <TOKEN>" \
  -F file=@app-release.apk -F origin=built -F tags=release,arm64
```

| Field | Meaning |
|---|---|
| `file` | the APK (required) |
| `original` | the original APK a patched file was made from |
| `origin` | `built`, `patched`, `original` or `unknown` (default) |
| `parent` | version id or SHA-256 of the original, when it is already stored |
| `tags` | comma separated; lowercase letters, digits and `. _ : -`, at most 32 of 64 bytes |
| `channel`, `changelog`, `notes` | free text; `channel` is `[a-z0-9._-]`, the others at most 64 KiB |
| `source_url` | an `http(s)` URL the file came from; stored, never fetched |
| `patch_info` | a JSON object: tool, patches, options, keystore alias (never the key) |
| `accept_new_signer` | store the file although the app's [signer policy](#signing-keys-and-policy) rejects its key |
| `no_original` | do not warn that the original of a patched file is not archived |

The answer is `201` with the version `id`, package, `version_code`, `sha256`, `signer_sha256`, `parent_id`, the generated `filename` and `warnings`. `409` means the file (or a file with the same package, `versionCode`, signer and ABI set) is already stored, or the signer was rejected; the body names the existing version or the key. `507` is the storage quota.

A version is identified by package, `versionCode`, signer and ABIs: a patched APK normally keeps the original's `versionCode`, so the same `versionCode` can exist once per signing key.

`POST /api/v1/repositories/REPO/apks/inspect` takes the same `file` and returns everything above without storing it, plus the stored duplicate and the latest stored version of the package.

Not supported yet: bundles (`.apks`, `.xapk`, `.apkm`) and split APKs are refused with `400`.

### Patched apps

Upload the patched file with the original it came from:

```sh
curl -X POST HOST/api/v1/repositories/REPO/apks -H "Authorization: Bearer <TOKEN>" \
  -F file=@patched.apk -F original=@original.apk -F origin=patched \
  -F 'patch_info={"tool":"revanced","patches":["..."]}'
```

The original is stored as its own version (`origin=original`) and the patched file's `parent_id` points to it; its publisher key is recorded as a known key. If the original is already stored it is reused. Without `original` or `parent`, a patched upload looks for a stored original with the same package and `versionCode`, and otherwise answers with the warning `original not archived`.

The version detail then shows whether the copy has the same signer as its parent and whether `content_digest` matches: a different signer with the same content is a re-signed copy, a different content is a modified one. A patched APK cannot be installed over the original; Android requires the same key.

## Signing keys and policy

The server never receives keystores or private keys; sign locally and upload the result. It only records public data: fingerprints and certificates.

Known keys (`/apks/signers`) carry a trust level: `mine`, `original`, `third_party` or `untrusted`, optionally scoped to one app. A key is registered from a SHA-256 fingerprint (with or without colons, as `keytool` prints it) or a PEM/DER certificate. Uploading an `original` registers its key as `original` unless the repository already knows it; a key you marked `mine` is never downgraded.

Each app has a signer policy (`PUT /apks/apps/<package>/meta`):

| Policy | Effect |
|---|---|
| `warn` (default) | warns when a `built` or `patched` file is not signed with a `mine` key, when any other file brings a key the app has not been signed with before (the first original of an app never does), and for any `untrusted` key |
| `allow` | no warnings about keys |
| `reject` | the same cases answer `409` with the key in the body, unless the upload sends `accept_new_signer=true` |

An unsigned file always warns. Warnings are stored with the version.

## Downloading

Routes under `HOST/repos/REPO`, readable with `read` access (an API token as Bearer, or as the Basic password):

| Route | Answer |
|---|---|
| `GET /apps/<package>/<versionCode>/<filename>` | the file; `HEAD`, `If-None-Match` and the usual presigned redirect are supported |
| `GET /apps/<package>/latest.apk` | `302` to the file of the highest `versionCode` |
| `GET /apps/<package>/latest.json` | `package_name`, `version_name`, `version_code`, `sha256`, `signer_sha256`, absolute `url`, `size`, `changelog`, `origin`, `channel`, SDKs, `abis` |
| `GET /apps/<package>/index.json` | the same entries for every stored version |

`latest.*` accept `signer` (SHA-256), `origin`, `channel`, `sdk` (a device API level: files with a higher `min_sdk` are skipped) and `abi` (files without native ABIs and files listing it match). `filename` is generated as `<package>-<versionCode>-<first 8 hex of the signer>.apk`, so variants of one `versionCode` have distinct URLs, and it must match exactly: a request for a key's file never answers with another key's file. Responses carry `X-OA-APK-Signer` and `X-Checksum-Sha256`.

```sh
curl -H "Authorization: Bearer <TOKEN>" 'HOST/repos/REPO/apps/org.example.app/latest.json?signer=<SHA-256>'
```

## Management API

Under `HOST/api/v1/repositories/REPO/apks`, authorized with the repository's own `read` (GET), `write` (POST, PUT) and `delete` (DELETE) permissions, not global `manage`, so a token with `write` can edit notes and tags. Resources are scoped to the repository in the URL; an id from another repository answers `404`. `PATCH` is not used.

| Route | Purpose |
|---|---|
| `GET apps` | apps with latest version, version count, origins, signers; filters `q` (package, titles in any language, notes, tags), `tag`, `origin`, `signer`; `sort` (`name`, `updated`, `versions`, `size`), `order`, `page`, `limit` |
| `GET apps/<package>`, `PUT apps/<package>/meta` | the app with the keys its versions use and how each is known; edit label, notes (Markdown), tags, signer policy |
| `GET apps/<package>/versions` | versions, newest `versionCode` first |
| `GET versions/<id>`, `PUT versions/<id>/meta`, `DELETE versions/<id>` | detail (manifest, signatures, native libraries, dependencies, zip checks, entries, icons, files, parent); edit notes, changelog, tags, channel, origin, parent, source URL, patch info, pinned; delete |
| `GET versions/<id>/manifest.xml` | the decoded manifest as plain text |
| `GET versions/<id>/icons`, `icons/<iconId>`, `icon?size=&role=` | icons and per-language titles; `icon` returns the smallest raster icon at least `size` pixels wide, else the largest |
| `GET/POST versions/<id>/files`, `GET/DELETE versions/<id>/files/<fid>` | attached files (`mapping`, `symbols`, `patch_bundle`, `patch_options`, `source`, `screenshot`, `original_apk`, `other`); served as downloads only |
| `GET/POST signers`, `PUT/DELETE signers/<id>` | known keys |
| `GET tags` | tags in use with counts |

`PUT .../meta` replaces only the fields sent; `tags` replaces the whole list.

## Web interface

An android repository replaces the generic package list with its own pages:

- **Packages** lists the apps with icon, latest version, origins, the key of the latest file and tags; search, origin filter, sort and tag chips narrow it. **Upload APK** analyses each file first (`POST .../apks/inspect`), shows the signer, SDK levels and warnings, and asks for the origin, the original of a patched file, tags and notes before storing it.
- **Signing keys** (`?tab=signers`) registers keys by fingerprint, `keytool -list -v` output or a PEM certificate. Keystores and private keys are refused.
- The app page (`/repos/REPO/packages/android/PACKAGE`) shows the keys the app is signed with, notes, tags, the signer policy and every version. The selected version (`?v=ID`) has tabs (`?tab=`): overview with the signature card, icons and titles, manifest, signature, native libraries, dependencies, provenance and zip entries.

## Behaviour that differs from other formats

- Deletes are permanent: the version, its icons and attached files go, and a blob is removed from storage once nothing else references it. There is no trash for android yet. Deleting an original leaves its patched copies, with their `parent_id` cleared.
- Storage quotas are enforced: each version, original and attached file takes one artifact slot and its size.
- Uploads and deletes emit `artifact.uploaded` and `artifact.deleted` webhook events with `format` `android`.
- Android repositories do not appear in the generic package listing, global search, backups, vulnerability scanning or SBOM generation yet. Use the management API.
- Notes, changelogs and titles are untrusted text from files and users; clients must render them as text. Icons are served with `nosniff` and a sandboxing CSP, and only raster images are served as images.

## Limits and configuration

| Setting | Default | Meaning |
|---|---|---|
| `OPENARTIFACT_ANDROID_MAX_UPLOAD_MB` | `2048` | largest accepted APK or attachment, in MiB; read from the environment only |

Uploads are buffered in a temporary file (`TMPDIR`) before parsing. The parser bounds entries, string pools, XML depth, icon count and sizes, and the bytes it reads from native libraries.

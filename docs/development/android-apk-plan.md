<!--
SPDX-FileCopyrightText: The OpenArtifact Authors

SPDX-License-Identifier: AGPL-3.0-or-later
-->

# Supporto Android APK: piano

Stato: **fase 0, backend della fase 1 e frontend della fase 2 completati; confronto/diff, bundle, CLI e indice F-Droid da fare.** Questa è una pagina di lavoro. Quando una fase è completata, il contenuto passa in `docs/formats/android.md` e questa pagina si accorcia.

### Stato dell'implementazione

Fatto e verificato (test di integrazione su Postgres 18; upload di 7 APK reali di F-Droid con fingerprint identici a `apksigner`; ri-firma di un APK reale con `apksigner` e upload con originale):

- `pkg/apk` (parser, firme v1-v3.1, icone, titoli, ELF) e `pkg/apk/apktest`.
- Migration `900_android.sql` + `900_android_indexes.no_tx.sql`, store in `internal/database/postgres_android{apps,misc}.go`, modelli in `models/android.go`.
- `internal/android`: upload (con `original` e `parent`), inspect, download, `latest.apk`/`latest.json`/`index.json`, app e versioni, icone, file collegati, chiavi note con policy per app, tag, note.
- Punti di contatto (un commit `seam(android):` ciascuno): `blobRefSpecs`, frammento OpenAPI, `validFormats`, `repoDataSegments`, `main.go`.
- `docs/formats/android.md`.
- Frontend (fase 2) in `web/src/features/android/`: pannello Apps con ricerca, filtri e tag, upload con anteprima da `inspect`, pagina app con riquadro chiavi, note e versioni, pagina versione a tab (panoramica con scheda Firma, icone e titoli, manifest, firma, librerie native, dipendenze, provenienza, file), pagina Chiavi; test in `__tests__/`. Verificato su un'istanza reale in tema chiaro e scuro. Punti di contatto: `formats.ts`, `en.ts`, `router/index.ts`, `RepoDetailPage.vue`, `RepoHeader.vue`, `installCommands.ts` + `clientsetup.go`, e `android` nell'enum `format` di `openapi.json` (serve ai tipi generati).

Deviazioni dal piano:

- **Eliminazione definitiva**, senza `deleted_at`: `TestTrashSpecsCoverEveryDeletedAtTable` impone che ogni tabella con quella colonna sia in `trashSpecs`. Il cestino è un lavoro della fase 7 (con una nuova migration).
- **OpenAPI**: l'API è descritta in `internal/android/openapi.json` e fusa all'avvio da `internal/api/openapi_fragments.go`; `openapi.json` non cambia. Il test di coerenza delle route registra anche l'handler android.
- **Download a nome esatto**: `<package>-<versionCode>-<8 hex del firmatario>.apk`; un nome che non coincide dà 404, mai il file di un'altra chiave.
- **Avvisi sul firmatario**: il primo `original` di un'app non avvisa; una chiave nuova per un'app già firmata sì (vedi `internal/android/policy.go`).
- I file Go non devono finire in `_android.go`: Go lo tratta come vincolo `GOOS=android` e il file non compila su Linux.
- Un'importazione da URL (`POST import`) e `GET diff` non ci sono ancora; nella UI mancano quindi il confronto (§11.5) e "importa da URL".
- Frontend: niente azioni di gruppo (tag/elimina su più app: non c'è un endpoint), niente QR dell'URL di download, niente voci della command palette; i tag-chip filtrano sui tag dell'app, non su quelli delle versioni. L'upload accetta solo `.apk` finché il backend non legge i bundle.

## 1. Obiettivo e caso d'uso

Un formato `android` hosted che funzioni da **archivio personale di app Android**: app compilate dal proprietario e app patchate partendo da un APK originale e ri-firmate con la sua chiave. Per ogni file l'archivio deve dire:

- cos'è: package, versione, livelli SDK, ABI, permessi, librerie native;
- **chi lo ha firmato**, con quale chiave, e se coincide con la chiave del proprietario o con quella dell'editore originale;
- da dove viene: compilato, patchato o originale, da quale originale deriva, con quale tool e quali patch;
- cosa è cambiato rispetto a un'altra versione o rispetto all'originale;
- note e tag scritti a mano.

Usi secondari: servire l'archivio ai dispositivi come repository F-Droid, fare da proxy di F-Droid, esporre un endpoint "ultima versione" per gli update checker come Obtainium.

Fuori scopo per le prime iterazioni: distribuzione su Google Play, flussi solo-AAB, controlli DRM o di licenza, scraping di siti terzi di download.

## 2. Decisioni prese

| # | Domanda | Decisione |
|---|---|---|
| 1 | Protocollo client | **API nativa + CLI `oa` per prime.** L'indice F-Droid viene dopo. |
| 2 | Bundle `.apkm` / `.xapk` / `.apks` | **Sì**, e vengono prima dell'indice F-Droid (sono la fonte tipica degli originali da patchare). |
| 3 | Firma | **Si firma in locale.** Il server non custodisce keystore. Il frontend deve però mostrare i dettagli della firma per capire con quale chiave si è firmato (§6). |
| 4 | Originale accanto al patchato | **Sì, sempre.** L'upload di un patchato accetta anche il file originale e li collega; senza originale né `parent` si mostra un avviso (§7). |
| 5 | Chi modifica tag e note | Chiunque abbia permesso `write` sul repository, non solo gli admin. In pratica userà solo il proprietario. |
| 6 | Parser | In casa su sola libreria standard, da confermare nello spike della fase 0. |
| 7 | Visibilità | Repository privato di default. |

## 3. Regole di progetto (compatibilità con i merge da upstream)

Si prevede un fork con merge occasionali da upstream. È una preferenza, non un vincolo: se una cosa è molto più semplice modificando un file condiviso, si modifica. Le regole sotto tengono il costo basso.

1. **Il codice nuovo sta in file e pacchetti nuovi.** I file condivisi si toccano solo nei punti elencati in §3.2.
2. **Un commit per ogni punto di contatto**, con prefisso `seam(android):`, così `git log --grep '^seam(android)'` elenca tutto ciò che può andare in conflitto.
3. **Niente riformattazioni, riordini o refactor** del codice upstream nel fork. I bug di upstream si correggono su upstream.
4. **I file generati si rigenerano, non si fondono.** In caso di conflitto su `internal/database/dbsqlc/`, `web/src/api/generated/` o `openapi.json` si prende la versione upstream, si riapplica la modifica e si lancia `make generate` / `bun run generate-api`.
5. **Si rispettano comunque gli invarianti di AGENTS.md** (header SPDX, hashing dei blob, scoping per repository, RBAC, `writeError`, JSON snake_case, `t()` per le stringhe), così il lavoro potrebbe in futuro essere proposto upstream, e allora i punti di contatto sparirebbero.
6. Setup git consigliato: `git remote add upstream <url>`, `git config rerere.enabled true`, merge di `upstream/main` a ogni tag di release, e tenere la parte di release specifica del fork (versione, nome immagine) in workflow nuovi invece di modificare `VERSION` o `release.yml`.

### 3.1 Dove vive il codice nuovo

| Area | Posizione | Note |
|---|---|---|
| Libreria di parsing APK | `pkg/apk/` | Go puro, senza import del repo; usata dal server e da `oa apk inspect`. |
| Handler del formato, indice F-Droid, download | `internal/format/android/` | Segue `internal/format/nuget`. |
| API di gestione (app, versioni, tag, note, diff) | `internal/android/` | Pacchetto proprio, registrato da `main.go` con una chiamata. |
| Store | `internal/database/postgres_android.go`, `internal/database/models/android.go` | File nuovi; metodi su `*PostgresDB`. SQL scritto a mano (il repo ne ha già in `postgres.go`), quindi niente rigenerazione di `dbsqlc/`. |
| Interfaccia dello store | In `internal/android`, sul modello di `ComponentNameLister` in `database/interfaces.go` | Evita di aggiungere metodi all'interfaccia `Database` e a `mock_db.go`. |
| Migration | `migrations/900_android.sql` (+ `900_android_indexes.no_tx.sql`) | Il test di embed richiede `^\d{3}_.*\.sql$`; `900` non collide con i numeri futuri di upstream e il migrator applica qualunque file non ancora applicato, in qualsiasi ordine. Dipende solo da `repositories` e `blobs`. |
| Frontend | `web/src/features/android/` (pagine, componenti, `api.ts`, `en.android.ts`, `routes.ts`) | Tipi propri invece del client generato; chiamate tramite `request()` di `web/src/api/client.ts`. |
| CLI | `cmd/oa/cmd_apk.go` | |
| Documentazione | `docs/formats/android.md` | |
| Configurazione | Chiavi solo-env lette in `internal/android` | Evita `internal/config`, `.env.example` e `openartifact.example.yaml`. Documentate nella pagina del formato. |
| Dipendenze | Nessuna nuova, se si sceglie il parser in casa (§5) | `go.mod` e `go.sum` restano senza conflitti. |

### 3.2 Punti di contatto (modifiche inevitabili a file condivisi)

I conteggi di righe sono stime da verificare durante l'implementazione.

| File | Modifica | Dimensione |
|---|---|---|
| `cmd/server/main.go` | costruire l'handler android; montare le route sul sub-router `/repos/{name}`; registrare l'API di gestione | ~4 righe |
| `internal/api/repositories.go` | aggiungere `"android": true` alla lista dei formati | 1 riga |
| `internal/middleware/rbac.go` | aggiungere `"apks"` a `repoDataSegments()` (riga ~541): così tutto `/api/v1/repositories/{name}/apks/...` è autorizzato con `read`/`write`/`delete` del repository invece di `manage` | 1 riga |
| `internal/api/openapi.json` + `openapi_routes_test.go` | le route registrate in `internal/api` devono essere nella spec. Si registra l'API android fuori da `internal/api` e si verifica come il test scopre i handler; altrimenti si fonde un frammento `openapi.android.json` al momento di servire la spec | 0-15 righe |
| `web/src/lib/formats.ts` | id formato, brand (`siAndroid`), ordinamento | ~3 righe |
| `web/src/i18n/en.ts` | aggiungere `enAndroid` al `catalog` | 2 righe |
| `web/src/router/index.ts` | aggiungere `androidRoutes` | 2 righe |
| `web/src/pages/RepoDetailPage.vue` | mostrare il pannello Apps quando `format === 'android'` | ~5 righe |
| `internal/clientsetup/clientsetup.go`, `web/src/lib/installCommands.ts` | snippet client (da tenere allineati, vedi AGENTS.md) | ~5 righe ciascuno |
| `docs/formats/README.md` | riga nella tabella | 1 riga |
| **Punti rimandati** (fase 7) | `ListRepoComponents`, `CountRepoComponents`, `search/executor.go`, `backup.go`, `worker/cleanup.go`, `trash_kinds.go`, `trash/release.go`, `vuln/scanner.go`, `sbom/generator.go`, `tests/testdata/*legacy*.sql` | più grandi; è qui che i conflitti sono più probabili |

Due vincoli emersi leggendo `rbac.go`, da rispettare nel disegno:

- **`PATCH` sotto un segmento "dati" ricade su `manage`.** Il `switch` interno accetta solo GET, HEAD, POST, PUT e DELETE. Siccome tag e note devono essere modificabili con il solo permesso `write`, le modifiche si fanno con `PUT` o `POST` (§9.3), non con `PATCH`.
- **Le pagine SPA sotto `/repos/{name}/...` sono ammesse solo per forme fisse** (`edit`, `settings`, `browse`, `graph`, `trash`, `packages/{format}/{pkg}`). Perciò la pagina di una app android usa la forma già ammessa `/repos/:name/packages/android/:pkg`, e versione, tab e confronto viaggiano nella query string (`?v=<id>&tab=signature`, `?compare=<a>,<b>`). Niente modifiche a `isSPARepoPath`.

Finché la fase 7 non è applicata, i repository android hanno elenco e ricerca propri nella UI Apps; i conteggi della dashboard generica, la ricerca globale e il backup non li includono. Prima della fase 7 va deciso se ne vale la pena.

## 4. Modello dati

Tutte le tabelle in `migrations/900_android.sql`, con `repository_id`; soft delete con `deleted_at` come gli altri formati; blob referenziati tramite `blob_id` (indirizzati per sha256; eliminazione solo con `storage.SafeDeleteBlob`; ogni `BlobStore.Put` con metadato `{"sha256": ...}`).

### `android_apps` (una riga per package name in un repository)

| Colonna | Note |
|---|---|
| `id`, `repository_id`, `package_name` | unico `(repository_id, package_name)` dove `deleted_at IS NULL` |
| `label_override`, `icon_override_blob_id` | le app patchate hanno spesso un'etichetta fuorviante |
| `notes` text | Markdown scritto a mano, max 64 KiB |
| `tags` text[] | normalizzati in minuscolo, `[a-z0-9._:-]`, max 32 tag da 64 byte; indice GIN |
| `upstream_kind`, `upstream_ref`, `upstream_latest` | tracciamento opzionale degli aggiornamenti (§15) |
| `created_at`, `updated_at`, `deleted_at` | default a now (regola dei timestamp di AGENTS.md) |

### `android_versions` (una riga per file archiviato)

| Gruppo | Colonne |
|---|---|
| Identità | `id`, `repository_id`, `app_id`, `package_name`, `version_code bigint` (64 bit, con `versionCodeMajor`), `version_name`, `file_kind` (`apk`, `apks`, `xapk`, `apkm`), `blob_id`, `filename` |
| Chiave di variante | `signer_sha256`, `abis text[]`, `variant_key` = hash di firmatario + ABI ordinate. Unico `(repository_id, package_name, version_code, variant_key)` dove `deleted_at IS NULL`. Un APK patchato di solito mantiene il `versionCode` dell'originale, quindi il firmatario deve far parte della chiave. |
| Manifest | `label` (titolo di default), `labels jsonb` (titolo per ogni lingua trovata, es. `{"": "App", "it": "App", "de": "App"}`), `launcher_label` (titolo dell'activity di lancio, se diverso), `min_sdk`, `target_sdk`, `compile_sdk`, `max_sdk`, `debuggable`, `permissions jsonb`, `features jsonb`, `components jsonb`, `app_flags jsonb`, `locales text[]`, `framework` (flutter, react-native, unity, native, ...) |
| Firma | `sig_schemes text[]`, `sig_verified bool`, `signature jsonb` (dettagli per firmatario, §6), `content_digest` (digest del contenuto firmato, indipendente dalla firma) |
| Codice nativo | `native_libs jsonb`, `page_size_16k bool` |
| Dipendenze | `dependencies jsonb` (coordinate Maven trovate in `META-INF`) |
| Provenienza | `origin` (`built`, `patched`, `original`, `unknown`), `parent_id` (FK su se stessa verso l'originale), `patch_info jsonb`, `source_url`, `original_sha256` |
| A mano | `changelog` text, `notes` text, `tags text[]`, `channel` (`stable`, `beta`, `nightly`, `archive`, testo libero), `pinned bool` |
| Statistiche | `download_count`, `created_at`, `updated_at`, `deleted_at`, `created_by` |

### `android_files` (file collegati a una versione)

`id, version_id, kind (mapping | symbols | patch_bundle | patch_options | source | screenshot | original_apk | other), name, blob_id, created_at`.

### `android_icons` (icone estratte dall'APK, più righe per versione)

`id, version_id, role (app | round | foreground | background | monochrome | other), density (ldpi, mdpi, hdpi, xhdpi, xxhdpi, xxxhdpi, anydpi, nodpi), width, height, mime (image/png | image/webp | image/svg+xml), source_path` (percorso nell'APK, es. `res/mipmap-xxxhdpi-v4/ic_launcher.png`), `resource_name` (es. `mipmap/ic_launcher`), `blob_id, created_at`. Le immagini identiche in più densità condividono lo stesso blob (indirizzato per sha256). Limiti: max 64 icone per versione e 2 MiB per icona; il resto diventa un avviso.

### `android_signers` (chiavi note)

`id, repository_id, sha256, label, trust (mine | original | third_party | untrusted), app_id nullable, notes, created_at`. Alimenta i badge della UI e la policy di §6.3.

### Altro

- L'`id` dell'artefatto per proprietà e webhook è l'UUID della versione, quindi `oa props` continua a funzionare.
- Il ramo in `ListRepoComponents` è rimandato (§3.2).
- La cache di autenticazione non richiede invalidazione: nessuna di queste tabelle è letta durante l'autenticazione.

## 5. Parsing APK (`pkg/apk`)

### 5.1 Cosa restituisce la libreria

Una funzione, `Inspect(ra io.ReaderAt, size int64, opts) (*Info, error)`, più `Diff(a, b *Info)`. Lo stesso pacchetto serve il server, `oa apk inspect` e l'endpoint di anteprima.

- **Livello zip:** `archive/zip`; elenco delle entry con CRC, dimensioni, metodo di compressione, allineamento; sha256 di entry selezionate; zip64 accettato.
- **Manifest:** decodifica del binary XML (`AndroidManifest.xml`) in una struttura tipizzata più il testo XML decodificato per la visualizzazione.
- **Risorse:** `resources.arsc` serve per titoli e icone (nel manifest sono riferimenti a risorse come `@string/app_name` e `@mipmap/ic_launcher`). Si implementano string pool, chunk package e type, e la lettura dei qualificatori di configurazione (lingua, regione, densità, versione API). Fallback: stringa grezza di `android:label`, poi il package name.
- **Titoli:** si risolve `android:label` dell'applicazione e quello dell'activity di lancio **per ogni configurazione di lingua** presente (`values`, `values-it`, `values-de`, `values-b+sr+Latn`, ...). Il risultato è una mappa lingua→titolo; la voce senza qualificatore è il titolo di default.
- **Icone:** si risolvono `android:icon` e `android:roundIcon` (applicazione e activity di lancio) e si estraggono **tutte le varianti disponibili**: ogni densità di `mipmap-*` e `drawable-*` (mdpi, hdpi, xhdpi, xxhdpi, xxxhdpi, anydpi, nodpi), con larghezza e altezza lette dall'header PNG/WebP. Per le icone adaptive (`mipmap-anydpi-v26/ic_launcher.xml`) si seguono i riferimenti ai layer `foreground`, `background` e `monochrome` e si estraggono come icone separate con il loro ruolo. Se un layer è un vector drawable binario, nella fase 1 si salva l'XML decodificato e si segna "non renderizzabile"; la conversione VectorDrawable→SVG (i path sono compatibili con SVG) è un'aggiunta della fase 3. Le immagini sono salvate come blob; qualunque errore diventa un avviso e non blocca mai l'upload.
- **Codice nativo:** per ogni `lib/<abi>/*.so` si legge l'header ELF con `debug/elf`: classe, macchina, soname, `DT_NEEDED`, build-id, strippato o no, allineamento dei `PT_LOAD`, e se la entry è non compressa e allineata a 16 KiB.
- **Framework:** Flutter (`libflutter.so`), React Native (`libreactnativejni.so`, `index.android.bundle`), Unity (`libunity.so`), Xamarin/.NET, Cordova, Kotlin Multiplatform, solo nativo.
- **Dipendenze:** `META-INF/*.version` (androidx e librerie Google), `META-INF/maven/**/pom.properties`, `META-INF/com/android/build/gradle/app-metadata.properties` (AGP e compile SDK). Il blocco di dependency info di AGP nel signing block è cifrato per Google e non si usa.
- **Bundle:** `.xapk` (`manifest.json`), `.apkm` (`info.json`), `.apks` (`toc.pb`): si legge l'APK base dentro il bundle, si elencano gli split (ABI, densità, lingua) e il bundle è una sola versione.

### 5.2 Robustezza

Gli APK patchati sono spesso malformati, quindi si tollera e si segnala invece di fallire:

- limiti rigidi: numero di entry, dimensione non compressa per entry, dimensione totale, dimensione dello string pool, profondità XML, ricorsione dei chunk;
- nessuna estrazione su disco, nessun uso dei nomi delle entry come percorsi;
- entry duplicate, CRC errati, byte prima dello zip (prefisso in stile Janus) diventano **avvisi** salvati con la versione; si rifiuta solo un file che non è uno zip o che non ha manifest;
- risultati parziali: etichetta o icona mancanti non respingono mai l'upload;
- target di fuzzing per i parser AXML, arsc e signing block; un corpus di APK reali (build F-Droid, un APK ricostruito con apktool, uno offuscato con AndResGuard, un bundle split) in `pkg/apk/testdata`.

### 5.3 Streaming e memoria

Lo zip richiede accesso casuale. L'upload viene scritto su un file temporaneo (come fanno gli handler deb/rpm/maven), poi `Inspect` gira sul file; il blob è salvato con metadato `{"sha256": ...}`. La dimensione massima è una costante (default 2 GiB) alzabile con una chiave env.

### 5.4 Libreria o codice in casa

Spike della fase 0: provare `shogo82148/androidbinary`, `avast/apkparser` e `avast/apkverifier` sul corpus e confrontare con `aapt2 dump badging` e `apksigner verify -v --print-certs` (usati solo sulla macchina di sviluppo; l'immagine runtime non ha shell). La decisione di partenza è codice in casa sulla libreria standard: AXML ~400 righe, sottoinsieme di arsc ~500, signing block ~400 con `crypto/x509`, `encoding/asn1`, `crypto/rsa|ecdsa`. Evita dipendenze nuove e dà controllo sulla tolleranza. Se lo spike mostra che una libreria copre il corpus, usarla come wrapper va bene.

## 6. Informazioni sulla firma

La firma è il dato più importante dell'archivio: serve a sapere con quale chiave si firma e a distinguere i propri build dagli originali. **La firma avviene sempre in locale**; il server la legge, la verifica e la mostra.

### 6.1 Cosa si estrae

Per ogni file e per ogni firmatario trovato:

| Voce | Fonte |
|---|---|
| Schemi presenti | v1 (JAR: `META-INF/*.SF`, `*.RSA|DSA|EC`), v2 (block id `0x7109871a`), v3 (`0xf05368c0`), v3.1 (`0x1b93ad61`). v4 è un file `.idsig` separato e non può stare in un APK; mostrato come "non applicabile". |
| Esito della verifica per schema | digest delle sezioni dello zip ricalcolati e firme controllate (RSA PKCS#1 v1.5 e PSS, ECDSA, DSA); errori e avvisi conservati come testo |
| Certificato | DN di subject e issuer, seriale, `notBefore`, `notAfter` (flag di scadenza; Android non controlla la scadenza all'installazione), algoritmo e dimensione della chiave pubblica, algoritmo di firma |
| Fingerprint | SHA-256 (principale, lo stesso valore che usano F-Droid, Obtainium e AppVerifier), SHA-1, MD5 |
| Catena | certificati presenti nei dati firmati, flag auto-firmato |
| Lineage v3 | rotazione delle chiavi `SigningCertificateLineage`, con min/max SDK di ogni firmatario |
| Algoritmi di digest | per schema (SHA-256, SHA-512, ...) |
| Flag | certificato di debug (`CN=Android Debug`), chiavi di test/piattaforma AOSP, RSA < 2048, solo SHA-1, certificato che scade prima del 2033 |
| Controlli zip | `zipalign` (4 byte), 16 KiB per i `.so` non compressi, `resources.arsc` non compresso con `targetSdk >= 30`, solo-v1 con `minSdk >= 24`, v2 obbligatoria con `targetSdk >= 30` |
| `content_digest` | digest del contenuto firmato indipendente dalla firma; due file con lo stesso valore sono lo stesso build firmato con chiavi diverse (copie ri-firmate) |

### 6.2 Vedere con quale chiave si firma (frontend)

Requisito esplicito: dalla UI deve essere immediato capire con quale chiave è firmato ogni file.

- **Scheda "Firma" in cima alla panoramica della versione**, sempre visibile: fingerprint SHA-256 con pulsante copia, etichetta della chiave se nota ("la mia chiave release", "editore originale", "sconosciuta"), schemi presenti con stato di verifica, avvisi.
- **Scheda "Firma" completa** nella pagina versione: tutti i dettagli di §6.1, il certificato per esteso, lineage, flag, controlli zip.
- **Colonna firmatario** nell'elenco versioni, con badge colorato per livello di fiducia e fingerprint abbreviato; raggruppamento per chiave.
- **Riepilogo chiavi dell'app**: nella pagina app, un riquadro "Chiavi usate" con ogni fingerprint, quante versioni ha firmato, prima e ultima data. Evidenzia se una versione `built` o `patched` è firmata con una chiave non marcata come propria.
- **Pagina Chiavi del repository** (§11.6): chiavi note, etichetta, livello di fiducia, app che le usano.
- **Registrare una chiave senza il keystore:** da una chiave già vista (pulsante "è la mia chiave"), da un certificato pubblico (`.pem`, `.crt`, `.cer`) caricato o incollato, o dalla riga SHA-256 incollata dall'output di `keytool -list -v`. Non si accettano mai keystore né chiavi private.
- **CLI:** `oa apk inspect FILE` stampa gli stessi dettagli offline, e accetta anche un certificato pubblico per calcolarne il fingerprint.

### 6.3 Confronto con l'originale

Se una versione ha `parent_id`, la UI mostra: stesso firmatario / firmatario diverso, e cosa significa ("un APK patchato non si installa sopra l'originale; va prima disinstallato e i dati vanno persi"). Mostra anche se `content_digest` coincide con quello del padre, cioè la differenza tra "solo ri-firmato" e "contenuto modificato".

### 6.4 Chiavi note e policy

- Caricare un `original` registra il suo firmatario in `android_signers` come `original` per quell'app.
- Il proprietario marca la propria chiave come `mine`; i badge dicono "tua chiave", "editore originale", "chiave sconosciuta".
- Policy per app (default: avviso): `consenti qualsiasi`, `avvisa su nuovo firmatario`, `rifiuta se non tra i firmatari noti`. Il rifiuto è `409 conflict` con i fingerprint nel corpo; la CLI può passare `--accept-new-signer`.
- Le varianti patchate usano legittimamente una chiave diversa dall'originale, quindi la policy si applica **per origine**: `patched` e `built` devono essere firmati con una chiave `mine`; `original` mantiene la chiave originale.

La firma lato server (keystore cifrati, endpoint che firma) **non è prevista**.

## 7. Provenienza e app patchate

Campi e comportamento per il flusso "patcho gli originali":

- `origin`: `original` (come scaricato), `patched`, `built`, `unknown`. Default dal flag della CLI; modificabile dalla UI.
- **L'originale si archivia sempre** accanto al patchato. L'upload di un `patched` accetta il campo `original` (il file originale): i due vengono salvati, collegati con `parent_id` e l'originale registra il proprio firmatario. Se si carica un `patched` senza `original` e senza `parent`, la risposta contiene un avviso e la UI lo mostra sulla versione ("originale non archiviato"); `--no-original` in CLI lo accetta esplicitamente.
- `parent_id`: l'originale da cui deriva, scelto tra le versioni esistenti dello stesso package, o proposto in automatico cercando `package_name` e `version_code` con `origin=original`. `original_sha256` resta anche se l'originale non è archiviato.
- `patch_info` (JSON): tool e versione (ReVanced CLI, apktool, smali a mano, ...), elenco delle patch con opzioni, alias del keystore usato (mai la chiave), data, testo libero. Un patch bundle e il file di opzioni si allegano come file collegati, così il build è riproducibile dall'archivio.
- `source_url`: da dove arriva l'originale.
- **Vista a coppie:** le versioni con lo stesso `package_name` e `version_code` si mostrano insieme, originale e patchato, con pulsante diff.
- **Rilevamento ri-firma** tramite `content_digest` (§6.1).
- **Diff rispetto all'originale** (§11.5): quali entry sono cambiate (`classes*.dex`, `resources.arsc`, `AndroidManifest.xml`, librerie aggiunte o rimosse), quali permessi o componenti sono stati aggiunti, cambi di SDK.
- Raggruppamento per firmatario nella pagina app: "i miei build" e "originali" sono visivamente separati.

## 8. Note e tag

- Tag a livello di **app** e di **versione**; note a livello di app e di versione; `changelog` è un campo separato della versione.
- Tag: normalizzati in scrittura, autocomplete dai tag già presenti nel repository, filtro per tag (AND di più tag) in elenco e ricerca, nuvola di tag nella pagina Apps. Nessun prefisso riservato; solo convenzione (`device:`, `patch:`, `base:`).
- Note: Markdown, renderizzato con sanitizzatore (niente HTML grezzo), modificabile inline con anteprima.
- `channel` e `pinned` sono campi separati, così possono guidare la retention e l'endpoint `latest` senza mescolarsi ai tag.
- Non è prevista nella prima versione la cronologia delle revisioni delle note; l'audit log registra già la modifica.
- Le proprietà generiche (`oa props`) continuano a funzionare e sono elencate nella pagina versione accanto ai tag.
- **Permessi:** la modifica richiede `write` sul repository (§3.2, §9.3).

## 9. API

### 9.1 Upload

`POST /api/v1/repositories/{repo}/apks` (multipart, come deb/rpm; richiede il segmento `apks` in `repoDataSegments()`, §3.2). Campi:

`file` (obbligatorio), `original` (file originale, per i patchati), `origin`, `parent` (id versione o sha256), `changelog`, `notes`, `tags` (separati da virgola), `channel`, `source_url`, `patch_info` (JSON), `accept_new_signer`, `no_original`. Risposta `201`: `{id, package_name, version_code, version_name, sha256, signer_sha256, parent_id, warnings: [...]}`. I duplicati (stessa chiave di variante) sono `409`. In UI più file vanno con chiamate ripetute.

`POST /api/v1/repositories/{repo}/apks/inspect` analizza senza salvare e restituisce le stesse informazioni più il diff rispetto all'ultima versione archiviata, così la UI può mostrare un'anteprima e chiedere conferma prima dell'upload vero.

I file grandi su S3/GCS potranno riusare in seguito il flusso presigned.

### 9.2 Route di lettura per i client (sotto `/repos/{name}`)

| Route | Scopo |
|---|---|
| `GET /apps/{pkg}/{versionCode}/{filename}` | download (token; anonimo solo se il repository è pubblico) |
| `GET /apps/{pkg}/latest.apk` | redirect al file migliore; query `sdk`, `abi`, `channel`, `signer`, `origin` |
| `GET /apps/{pkg}/latest.json` | `{version_name, version_code, sha256, signer_sha256, url, size, changelog}` per gli update checker |
| `GET /apps/{pkg}/index.json` | tutte le versioni |
| `GET /index-v2.json`, `/entry.jar`, `/index-v1.jar` | indice F-Droid (fase 6) |

Le risposte seguono la regola di cache HTTP (`private, no-cache`, salvo lettura anonima di repository pubblico) e portano `X-OA-APK-Signer` e un `ETag` basato sullo sha256. `HEAD` supportato.

### 9.3 Gestione (`/api/v1/repositories/{repo}/apks/...`)

Tutto sotto il segmento `apks`, quindi autorizzato con `read` (GET), `write` (POST, PUT) e `delete` (DELETE) del repository. **Non si usa `PATCH`** (§3.2).

| Metodo e percorso | Scopo |
|---|---|
| `GET apps?q=&tag=&origin=&signer=&sort=&page=&limit=` | elenco app con ultima versione, numero di versioni, tag |
| `GET apps/{pkg}` | dettaglio app, con riepilogo delle chiavi usate |
| `PUT apps/{pkg}/meta` | etichetta, note, tag, impostazioni upstream (sostituisce i campi inviati) |
| `GET apps/{pkg}/versions?...` | versioni con filtri |
| `GET versions/{id}` | dettaglio completo (manifest, firma, librerie native, dipendenze, provenienza, avvisi) |
| `PUT versions/{id}/meta` | note, changelog, tag, canale, origine, parent, source URL, patch info, pinned |
| `DELETE versions/{id}` | soft delete; la versione deve appartenere al repository dell'URL |
| `GET versions/{id}/manifest.xml` | testo del manifest decodificato |
| `GET versions/{id}/icons` | elenco di tutte le icone della versione (ruolo, densità, dimensioni, mime, percorso nell'APK, URL) e dei titoli per lingua |
| `GET versions/{id}/icons/{iconId}` | immagine della singola icona (con `ETag` dello sha256) |
| `GET versions/{id}/icon?size=&role=` | scorciatoia: l'icona più adatta alla dimensione richiesta in pixel (default: la più grande del ruolo `app`); usata da elenchi e miniature |
| `GET diff?a=<id>&b=<id>` | diff strutturato |
| `GET versions/{id}/files`, `POST versions/{id}/files`, `DELETE versions/{id}/files/{fid}` | file collegati |
| `GET signers`, `POST signers`, `PUT signers/{id}`, `DELETE signers/{id}` | chiavi note (anche da certificato pubblico o fingerprint) |
| `GET tags` | tag in uso con conteggi, per l'autocomplete |
| `POST import` | scarica un file da URL (client SSRF-safe, `internal/netutil`) per il pulsante "importa da F-Droid o GitHub" |

JSON in snake_case; errori con `writeError` e i codici condivisi; nulla di ciò che arriva dal database o da upstream viene riportato al client. Tutte le route compaiono nella spec OpenAPI (vedi la nota sul punto di contatto).

## 10. F-Droid e altri client

### 10.1 F-Droid come proxy: nessun codice necessario

Un **proxy raw** copia già i file per percorso. Si crea un repository di formato `raw`, tipo `proxy`, upstream `https://f-droid.org/repo` (oppure `https://apt.izzysoft.de/fdroid/repo`, o `https://f-droid.org/archive` per le versioni vecchie), e si usa `HOST/repos/raw/REPO` come indirizzo del repository in un client F-Droid. L'indice è servito invariato, quindi firma e fingerprint di upstream restano validi. Da verificare nella fase 0: il TTL dell'indice (`cache_ttl_seconds`, default 1 h), i limiti di dimensione del proxy, e che il client accetti l'URL.

In seguito un tipo di proxy android dedicato potrebbe analizzare gli APK in cache ed elencarli nella UI, e un pulsante "importa nel mio archivio" copierebbe un file upstream in un repository hosted come `origin=original` con `source_url`. Richiede una voce in `repotype` ed è opzionale.

### 10.2 Repository hosted come repository F-Droid (fase 6)

- `index-v2.json` più `entry.jar` firmato (formato nuovo) e `index-v1.jar` (client più vecchi). La firma è in stile JAR (`MANIFEST.MF`, `.SF`, firma PKCS#7) con una **chiave del repository** generata sul server, salvata cifrata con `internal/crypto`, con una pagina admin che mostra il fingerprint SHA-256 e un QR `fdroidrepos://` per aggiungere il repository. Questa chiave è diversa dalle chiavi con cui si firmano le app, che restano in locale.
- I metadati vengono dall'app (etichetta, note come descrizione, tag come categorie, icona, screenshot dai file collegati). `nativecode`, `minSdkVersion`, `targetSdkVersion`, `uses-permission`, `sig`, `signer` si leggono dalla versione archiviata. Più varianti con lo stesso `versionCode` sono elencate con le loro ABI.
- Repository privati: i client F-Droid accettano le credenziali nell'URL; il token va nella posizione della password Basic (regola client-auth di AGENTS.md).
- La rigenerazione passa dal `RegenQueue` con coalescing, mai una goroutine per upload.

### 10.3 Obtainium e simili

`latest.json` / `latest.apk` (§9.2) più un URL di download stabile bastano per la sorgente HTML/regex o APK diretto di Obtainium. Documentato con un esempio.

## 11. Frontend (completo)

Tutto in `web/src/features/android/`, con i componenti esistenti (`PageHeader`, `DataTable`, `FormatBadge`, `StatusBadge`, `ConfirmDialog`, `AppButton`, `EmptyState`, `Pagination`, `ToastHost`), `formatDate`/`formatDateTime`, `t()` e le regole di stile (niente gradienti, glow, blur, etichette maiuscole spaziate). Tema chiaro e scuro verificati su ogni pagina.

### 11.1 Pagina del repository: pannello Apps

Mostrato in `RepoDetailPage` quando il formato è `android`. Tabella o griglia di schede (scelta ricordata per visualizzatore in local storage); ogni riga: icona (miniatura dalla variante più vicina alla dimensione mostrata), titolo nella lingua del browser con ripiego sul default, package name, ultima versione, numero di versioni, origini presenti (original / patched / built), badge del firmatario, tag, ultimo aggiornamento, dimensione. Barra degli strumenti: ricerca (etichetta, package, note, tag), chip dei tag con conteggi, filtro per origine, filtro per firmatario, filtro "compatibile con SDK/ABI", ordinamento (nome, aggiornamento, versioni, dimensione). Azioni di gruppo: aggiungi o rimuovi tag, elimina. Lo stato vuoto spiega upload e proxy F-Droid.

### 11.2 Upload

Zona di upload dedicata (il generico `PackageUploadZone` non viene toccato): si trascinano uno o più `.apk`, `.apks`, `.xapk`, `.apkm`. Ogni file passa da `inspect` e mostra una scheda di anteprima (icone in varie dimensioni, titolo e titoli per lingua, versione, SDK, ABI, **firmatario con badge e fingerprint**, avvisi, diff rispetto all'ultima versione archiviata, avviso di duplicato). Per ogni file o per l'intero lotto: origine, **campo "file originale"** per i patchati (con avviso se manca), parent (proposto), tag (autocomplete), canale, changelog, note, source URL, patch info. Conferma con barra di avanzamento per file; in caso di errore il modulo resta compilato. Presente anche "importa da URL".

### 11.3 Pagina app `/repos/:name/packages/android/:pkg`

- Intestazione: icona grande, titolo (con i titoli delle altre lingue in un menu a comparsa), package name, pulsanti copia, tag come chip modificabili, badge "aggiornamento upstream disponibile".
- **Riquadro "Chiavi usate"** (§6.2).
- Note: Markdown, modifica inline con anteprima e salvataggio.
- Elenco versioni raggruppato per `versionCode`, con originale e patchato affiancati; colonne: nome e codice versione, origine, canale, ABI, SDK min/target, dimensione, badge firmatario (fingerprint abbreviato, copia), data, download, stella pinned. Filtri e ordinamento; selezionando due righe si apre il confronto.
- Cronologia dei changelog.
- Azioni: scarica, copia URL `latest`, copia comando `adb install`, elimina (con conferma), pin.

### 11.4 Pagina versione (tab, via `?v=<id>&tab=...`)

| Tab | Contenuto |
|---|---|
| Panoramica | **scheda Firma in evidenza**, sha256, dimensione, download, copia URL, QR dell'URL di download, comandi `adb install` / `adb install-multiple`, avvisi, tag, note, changelog (modificabili inline), canale, origine, source URL |
| Icone e titoli | **galleria di tutte le icone** estratte dall'APK, raggruppate per ruolo (app, round, foreground, background, monochrome): ogni miniatura con densità, dimensioni in pixel, formato, percorso nell'APK e pulsanti scarica / copia sha256; selettore di dimensione di anteprima (48, 96, 192, 512 px, rendering nitido senza smussatura per vedere i pixel reali); per le icone adaptive anteprima composta (background + foreground) con maschera selezionabile (cerchio, squircle, quadrato arrotondato) e variante monochrome; fondo chiaro/scuro commutabile per vedere la trasparenza. **Tabella dei titoli** per lingua (codice lingua, titolo, indicazione di quello di default e del titolo dell'activity di lancio se diverso), con ricerca e copia |
| Manifest | package, versioni, livelli SDK, flag dell'applicazione (debuggable, allowBackup, cleartext traffic, network security config); permessi raggruppati normal / dangerous / signature / custom con breve descrizione e ricerca; feature; componenti (activity, service, receiver, provider) con informazioni su exported e permesso; intent filter inclusi i deep link; `queries`; visualizzatore del XML decodificato con ricerca |
| Firma | schemi con stato di verifica, dettagli del certificato per firmatario (§6.1), copia del fingerprint, lineage, flag (chiave di debug, chiave debole, scadenza), badge della chiave nota e pulsante "marca questa chiave come mia / originale / non fidata", controlli zip |
| Librerie native | tabella per ABI: file, dimensione, soname, librerie richieste, build-id, strippato, allineato a 16 KiB; framework rilevato |
| Dipendenze | coordinate Maven trovate, collegamento ai risultati di vulnerabilità e licenza quando la fase 7 è applicata; esportazione SBOM (CycloneDX col generatore esistente, quando collegato) |
| Provenienza | origine, link al parent, sha256 dell'originale, tool e patch, source URL, file collegati con upload ed eliminazione |
| File | entry dello zip con dimensione e CRC; filtro e ricerca; dimensione per categoria (dex, risorse, assets, lib, altro) come grafico a barre con `chartColor` |

### 11.5 Pagina di confronto `/repos/:name/packages/android/:pkg?compare=<a>,<b>`

Due selettori di versione (default: versione contro il suo parent, o contro la precedente). Sezioni: riepilogo (versione, differenza di dimensione, firmatario uguale o diverso, `content_digest` uguale o no); diff del manifest (permessi, componenti, SDK, flag) con aggiunti e rimossi distinti da icona oltre che dal colore; librerie native aggiunte, rimosse, cambiate; entry dello zip aggiunte, rimosse, cambiate (per CRC) con dimensioni; dimensione per categoria prima e dopo. Copia del riepilogo in Markdown. Un pulsante "genera bozza di changelog" riempie il campo changelog dal diff (e, se configurato, dall'AI analysis worker).

### 11.6 Pagina Chiavi

Per repository (tab `?tab=signers` della pagina del repository): chiavi note con etichetta, livello di fiducia, app che le usano, note; crea (da certificato, fingerprint o chiave già vista), modifica, elimina. Segnala le app le cui versioni usano più chiavi di livelli di fiducia diversi.

### 11.7 Integrazione globale

Voci nella command palette (app, tag, "carica APK"), una sezione Apps nella ricerca finché non esiste il punto di contatto della ricerca generica, chiavi i18n in `en.android.ts`, test unitari per store e componenti (`bun test`), typecheck, e una verifica manuale su un'istanza reale in entrambi i temi.

## 12. CLI (`oa apk`)

| Comando | Scopo |
|---|---|
| `oa apk inspect FILE [--extract-icons DIR]` | offline: manifest, titoli per lingua, elenco icone (densità, dimensioni), SDK, ABI, permessi, firma completa (usa `pkg/apk`, senza server); `--extract-icons` salva tutte le icone in una cartella; accetta anche un certificato pubblico per stamparne il fingerprint |
| `oa apk upload REPO FILE... [--origin patched --original FILE --parent SHA --tag x --note ... --changelog file --source-url U --patch-info file.json --channel beta --no-original --accept-new-signer]` | upload |
| `oa apk diff A B` | diff offline di due file locali, o di versioni archiviate |
| `oa apk list REPO [--tag x --origin patched]` | elenco |
| `oa apk tag REPO PKG[:CODE] +tag -tag` | modifica i tag |
| `oa apk note REPO PKG[:CODE] [--file notes.md]` | modifica le note |
| `oa apk download REPO PKG [--latest --sdk N --abi A]` | download |
| `oa apk signer list|add|label` | chiavi note |

## 13. Sicurezza

- Il server non installa né esegue nulla di ciò che sta in un APK; il parsing è limitato (§5.2) e sottoposto a fuzzing.
- Ogni handler che carica una versione per id verifica che appartenga al repository dell'URL.
- Note e changelog sono Markdown renderizzato con sanitizzatore; le etichette dei manifest sono testo, mai HTML.
- L'import da URL usa il client SSRF-safe; i segreti (la chiave del repository F-Droid) sono cifrati con `internal/crypto` e mai restituiti.
- Il server non riceve né conserva mai chiavi private o keystore: solo certificati pubblici e fingerprint.
- I download seguono il permesso di lettura esistente; i repository `public` permettono solo la lettura anonima.
- Le app patchate sono elencate come tali, con un avviso visibile che la firma differisce da quella dell'editore originale.
- La chiave del repository F-Droid è un segreto di alto valore: va salvata a parte e va documentata la rotazione.
- Prima di rendere pubblico un repository, verificare la posizione legale della redistribuzione delle app.

## 14. Fasi, stime e criteri di completamento

Stime per uno sviluppatore, approssimative, con parser in casa.

| Fase | Contenuto | Stima | Completa quando |
|---|---|---|---|
| 0. Spike | corpus di APK reali, decisione libreria o codice in casa, prova del proxy raw verso F-Droid con un client reale, verifica di come `openapi_routes_test` scopre i handler | 1-2 giorni | decisione registrata in questa pagina; il proxy funziona con un client reale |
| 1. Backend core | `pkg/apk` (zip, AXML, arsc, signing block, ELF, dipendenze), migration, store, upload con originale, inspect, download, `latest`, tag, note, provenienza, file collegati, chiavi note, API di gestione, pagina di documentazione | 9-11 giorni | l'upload di un APK reale salva metadati e firmatario corretti; `go test` e `make lint` passano; i target di fuzzing girano |
| 2. Frontend | §11.1-11.4, 11.6, 11.7, con la scheda Firma e il riquadro Chiavi | 7-9 giorni | tutte le pagine usabili su un'istanza reale, typecheck e test verdi, temi chiaro e scuro verificati |
| 3. Diff, analisi e bundle | `Diff`, pagina di confronto, librerie native in dettaglio, dipendenze, bozza di changelog, supporto `.apkm`/`.xapk`/`.apks` | 6-8 giorni | il diff patchato contro originale è corretto su esempi reali; un bundle APKMirror viene archiviato come una sola versione |
| 4. CLI | `oa apk ...` | 2 giorni | comandi coperti da test con fake, come la CLI esistente |
| 5. Rifinitura e verifica | documentazione `docs/formats/android.md`, `docs/README.md`, verifica end to end con docker compose e client reali (`curl`, `adb`) | 2 giorni | percorso completo provato: carica originale + patchato, vedi la firma, confronta, scarica |
| 6. Indice F-Droid | `index-v2` / `entry.jar` / `index-v1.jar` firmati, pagina della chiave del repository | 5-6 giorni | il client F-Droid installa un'app dal repository e accetta il fingerprint |
| 7. Punti rimandati | rami in elenco e ricerca generici, backup e restore, trash e cleanup, mapping vuln, SBOM e licenza | 3-4 giorni | dashboard, ricerca e backup generici includono android; test di equivalenza aggiornati |
| 8. Tracker upstream | controllo opzionale degli aggiornamenti (F-Droid, release GitHub) con job cron, badge "aggiornamento disponibile", evento webhook | 2-3 giorni | |

Totali indicativi: archivio utilizzabile con UI, diff, bundle e CLI (fasi 0-5) circa 5 settimane; indice F-Droid circa 1 settimana in più.

## 15. Altre idee (backlog)

- **Tracker upstream** (fase 8): per app, una sorgente upstream (id pacchetto F-Droid o repository GitHub) controllata da `internal/cron`; la UI mostra "l'originale è avanti di 2 versioni rispetto al tuo patchato". Non si fa scraping dei siti di download di APK.
- **Rilevamento ri-firma** tramite `content_digest`, già parte della fase 1.
- **Retention:** tenere le ultime N versioni per app, mai eliminare ciò che è `pinned` o con tag `keep`, tramite il cleanup worker.
- **Ricerca per capacità:** "quali app richiedono SMS", "quali includono `libssl` più vecchia di X", "quali hanno ancora firma solo SHA-1", "quali non sono allineate a 16 KiB".
- **Webhook** `android.version.uploaded` con il dispatcher esistente.
- **Pagina di installazione** con link a scadenza (QR, sha256, firmatario) per mandare un build a un tester.
- **Avviso di regressione di dimensione** tra versioni consecutive oltre una soglia.
- **Scansione di segreti** su stringhe e risorse con `internal/secrets` (chiavi API, `google-services.json`).
- **Lint del manifest** come regole di quarantena: `debuggable=true` in un canale release, cleartext traffic, componenti esportati senza permesso, certificato di debug.
- **Delta tra APK** con `pkg/delta` per dispositivi con poca banda (esotico, nessun client lo legge oggi).
- **Verifica di riproducibilità:** confrontare un APK ricostruito con quello archiviato ignorando la firma (`content_digest`).
- **Mapping e simboli** caricati per versione per la deoffuscazione dei crash.
- **Filtro di compatibilità del dispositivo** nella UI: si imposta una volta "il mio dispositivo = SDK 34, arm64-v8a" e si nascondono i file incompatibili.
- **Tag e note generici per tutti i formati:** se funzionano bene per android, le stesse colonne si potrebbero generalizzare (sarebbe una modifica upstream).

## 16. Da verificare durante l'implementazione

1. Come `openapi_routes_test` scopre le route, per decidere se serve il frammento `openapi.android.json` (§3.2).
2. Se il proxy raw regge i file di F-Droid più grandi e il TTL dell'indice (§10.1).
3. Se un `PUT` per le modifiche di metadati è accettato da proxy e CORS esistenti senza altre modifiche.
4. Se i test che elencano i formati (`FormatBadge.test.ts`, matrici di formati in `tests/`) richiedono aggiornamenti oltre ai punti di contatto elencati.
5. La dimensione massima degli upload (§5.3) rispetto ai limiti di quota e storage esistenti.
6. Casi limite di titoli e icone nel corpus dello spike: titoli con riferimenti annidati, app senza `android:icon`, icone solo adaptive con vector drawable, APK con risorse offuscate o con icone in `drawable` invece che in `mipmap`, WebP animati.

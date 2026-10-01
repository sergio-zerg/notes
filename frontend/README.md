# 📝 Notes — Flutter client

A Flutter front end for the [Notes API](../backend/README.md): sign in with an API
key, search and page through notes, create and edit them.

## 🏗 Architecture

Dependencies point inwards. `domain` imports no dio, no JSON and no Flutter;
`data` implements the interface `domain` declares; `presentation` talks to
controllers that never see a transport.

```
lib/
  main.dart                  entry point
  app/                       MaterialApp, routes, theme and its controller
  core/
    config/                  API address and limits, with per-platform defaults
    network/                 dio setup, request guard, ApiException
    storage/                 API key and theme preference, behind interfaces
    utils/                   Debouncer
    widgets/                 shared widgets with no feature of their own
  features/
    auth/
      application/           AuthController — session, sign in and out
      presentation/          LoginScreen
    notes/
      domain/                Note, NotesRepository, validation
      data/                  NoteModel, NotesApiRepository
      application/           NotesListController, noteControllerProvider
      presentation/          NotesListScreen, NoteEditorScreen, NoteCard
```

**Why the layers are worth the ceremony.** The domain declares
`NotesRepository`, so a controller asks for notes and receives notes. Swapping
REST for gRPC touches `data/` alone. Validation lives in `domain/note_validation.dart`
and mirrors the server's rules — including its rune-based length — because a
client that measures a title in UTF-16 code units accepts 100 emoji and then
loses the note on save.

## 🔑 Authentication

The key is held in the platform keystore behind `ApiKeyStore` and cached in
memory for the session, so the dio interceptor attaches it without a keystore
round trip on every request.

Signing in is a **request, not a local check**. The server is the only party that
knows whether a key is still configured, and a key revoked server-side would
otherwise be accepted here and fail every later call. That check reports one of
three verdicts:

| Verdict | Meaning | Session | Stored key |
|---------|---------|---------|-----------|
| `accepted` | The server took it | authenticated | written |
| `rejected` | `401` or `403` | unauthenticated | **cleared** |
| `undecided` | Timeout, DNS failure, `500` | unchanged | kept |

Collapsing the last two into a boolean is the mistake worth naming: it either
signs the user in to an app that cannot load anything, or blames their key for
the server's outage. The session is only moved to `unauthenticated` on an actual
rejection, and a rejected key is erased from the keystore — otherwise the next
cold start reads it back, marks the session authenticated and fails every request
with it.

## 🚦 Paging and list ordering

The server orders notes by ascending `id` and pages with `OFFSET`. Three things
follow, and all three are load-bearing:

- **The list keeps the server's order.** A note created or edited is placed where
  that ordering puts it, not at the top of the screen. Reordering locally means
  the list no longer describes the window the next request reads, and the next
  page repeats or skips a row.
- **The cursor is not the list length.** `nextOffset` is tracked separately,
  because a delete shifts every later server offset down by one. Deriving the
  offset from what is on screen skips exactly one note per delete.
- **Pages are deduplicated on append.** A note created while a page was open takes
  the slot at the cursor and comes back in the next page; without deduplication it
  would appear twice.

`hasMore` is recorded when a page arrives rather than computed from the length,
for the same reason: the endpoint reports no total count, so a short page is the
only end-of-list signal there is.

## 🌐 Error handling

`core/network/api_client.dart` maps a dio failure onto a sealed `ApiException`,
which the repository passes up unchanged:

| Case | Status | Retrying |
|------|--------|----------|
| `ApiExceptionUnauthorized` | 401 | no |
| `ApiExceptionValidation` | 400 | no |
| `ApiExceptionNotFound` | 404 | no |
| `ApiExceptionPayloadTooLarge` | 413 | no |
| `ApiExceptionRateLimited` | 429 | yes, after `Retry-After` |
| `ApiExceptionServer` | 5xx | yes |
| `ApiExceptionNetwork` | no response | yes |

Every one carries a `message` written for a person. Screens show that message and
never the error object: interpolating one prints its Dart type, so the user sees
`ApiExceptionNotFound: This note no longer exists.` instead of the sentence.

`ApiEndpointHint` shows the base URL under any connection failure, because a wrong
address is the most common first-run problem and an opaque "cannot reach the
server" with no address gives nobody anything to act on.

## ▶ Running

```bash
flutter pub get
flutter run
```

The API address defaults per platform, so no configuration is needed for the
usual cases:

| Target | Address |
|--------|---------|
| Web, iOS simulator, desktop | `http://localhost:8080` |
| Android emulator | `http://10.0.2.2:8080` |

Override for a physical device or a non-local backend:

```bash
flutter run --dart-define=API_BASE_URL=http://192.168.1.10:8080
```

A web build also needs the server to permit its origin — see
[CORS](../backend/README.md#-cors) — because `flutter run -d chrome` serves on a
different port than the API and the browser hides the response otherwise.

Start the backend first:

```bash
cd ../backend && docker compose up -d db
API_KEYS=dev-secret-key go run ./cmd/api
```

## 🐳 In Docker

A release build of this app, served by nginx, comes up with the rest of the stack
from the parent directory:

```bash
cd ..
./up.sh              # web on :8081, API on :8080
./up.sh rebuild      # rebuild after changing lib/
```

`frontend/Dockerfile` builds the bundle with the Flutter SDK and keeps only the
static output; `nginx.conf` serves it with hashed assets cached immutably and
every unknown path falling back to `index.html`, because the routes live in
go_router and have no file behind them.

There is no ready-made image to start from: the newest published Flutter image
carries Dart 3.12, this project requires `^3.13.4`, and no tag satisfies that.
The SDK is fetched from the official release archive with its checksum pinned,
so upgrading means changing `FLUTTER_VERSION` and `FLUTTER_SHA256` together.
Flutter also ships no arm64 Linux SDK, so the build stage is pinned to
`linux/amd64` — correct on both architectures, but emulated on Apple Silicon,
which is where the time goes on a cold build.

**The API address is compiled in.** `--dart-define=API_BASE_URL` is baked into
the bundle at build time, so a served build cannot be repointed afterwards:

```bash
WEB_API_BASE_URL=http://192.168.1.10:8080 ./up.sh rebuild   # phone on the LAN
```

A browser also has to be allowed by CORS — the default `*` already covers
`localhost:8081`, but a pinned allow-list needs that origin added.

## 🧪 Testing

```bash
flutter analyze
flutter test
```

161 tests, no mocks of the framework: repositories are mocked behind their real
interfaces, and widget tests drive real widgets through `pumpWidget`. The list
screen tests build the app's actual route table, because the list navigates with
`context.push` and a screen tested outside a router would be testing a second,
hand-built navigation path that ships to no one.

## 📦 Dependencies

`dio` for HTTP, `flutter_riverpod` for state, `go_router` for routing,
`flutter_secure_storage` for the key, `mocktail` for tests.

`intl` is declared but unreferenced, and `cupertino_icons` is unused — both are
left alone rather than removed on a hunch about what they were for.
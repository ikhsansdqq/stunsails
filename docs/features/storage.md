# Storage and cleanup

**Storage** shows Docker's disk use: images, containers, volumes, build cache,
and networks.

## Removing

- Images and networks can be removed one at a time.
- **Volumes are deleted one at a time, never pruned in bulk.** A volume holds
  data; a bulk prune is how databases disappear.

## Prunes

| Kind | Clears |
|---|---|
| `containers` | stopped containers |
| `images` | images no container uses, tagged or not |
| `dangling` | untagged images only |
| `build-cache` | the builder cache |
| `networks` | unused networks |

## Scheduled cleanup

Set in **Settings → Cleanup**: a weekday, an hour (server time), and which
kinds to prune. The default is Sunday at 04:00, build cache and untagged images.
Each run is recorded in Activity and can send a summary alert.

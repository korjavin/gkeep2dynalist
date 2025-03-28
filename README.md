# Google Keep to Dynalist Migration Tool

A command-line tool to migrate Google Keep notes from a Google Takeout export to Dynalist.

## Features

- Processes Google Keep notes from a Google Takeout export
- Uploads attachments (images, etc.) to Cloudflare R2 storage
- Creates Dynalist inbox items with:
  - Original note title and content
  - Links to uploaded attachments
  - Labels converted to hashtags
- Docker support for easy deployment

## Prerequisites

- Google Keep Takeout export (download from [Google Takeout](https://takeout.google.com/))
- Dynalist API token
- Cloudflare R2 account (optional, for attachment uploads)

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `DYNALIST_TOKEN` | Your Dynalist API token | Yes |
| `CF_ACCOUNT_ID` | Cloudflare account ID | For media uploads |
| `CF_ACCESS_KEY_ID` | Cloudflare R2 access key ID | For media uploads |
| `CF_ACCESS_KEY_SECRET` | Cloudflare R2 access key secret | For media uploads |
| `CF_BUCKET_NAME` | Cloudflare R2 bucket name | For media uploads |

## Usage

```bash
# With Docker
docker run --rm -it \
  -e DYNALIST_TOKEN=your_token \
  -e CF_ACCOUNT_ID=your_account_id \
  -e CF_ACCESS_KEY_ID=your_access_key \
  -e CF_ACCESS_KEY_SECRET=your_secret \
  -e CF_BUCKET_NAME=your_bucket \
  -v /path/to/takeout:/takeout \
  gkeep2dynalist /takeout/Takeout/Keep

# Without Docker
export DYNALIST_TOKEN=your_token
export CF_ACCOUNT_ID=your_account_id
export CF_ACCESS_KEY_ID=your_access_key
export CF_ACCESS_KEY_SECRET=your_secret
export CF_BUCKET_NAME=your_bucket
./gkeep2dynalist /path/to/takeout/Takeout/Keep
```

## How It Works

1. The tool scans the specified directory for Google Keep JSON files
2. For each note:
   - Parses the JSON data
   - If attachments exist, uploads them to Cloudflare R2
   - Converts Google Keep labels to hashtags
   - Creates a Dynalist inbox item with the note content and attachment links

## Building from Source

```bash
git clone https://github.com/korjavin/gkeep2dynalist.git
cd gkeep2dynalist
go build
```

## Docker

```bash
# Build the Docker image
docker build -t gkeep2dynalist .

# Run the Docker image
docker run --rm -it \
  -e DYNALIST_TOKEN=your_token \
  -e CF_ACCOUNT_ID=your_account_id \
  -e CF_ACCESS_KEY_ID=your_access_key \
  -e CF_ACCESS_KEY_SECRET=your_secret \
  -e CF_BUCKET_NAME=your_bucket \
  -v /path/to/takeout:/takeout \
  gkeep2dynalist /takeout/Takeout/Keep
```

## License

MIT
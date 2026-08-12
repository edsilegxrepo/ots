> OTS uses two different formats to store secrets under the hood. Both of them can be read and written by the frontend implementation as well as by the `ots-cli` application.

## Simple Format

The simple format is the format used by OTS since day one. Modern OTS clients use **AES-256-GCM** encryption by default (via Web Crypto API and Go standard `crypto/cipher`), while maintaining full backward compatibility with OpenSSL **AES-256-CBC** PBKDF2 SHA-512 encrypted payloads. This format is preferred for backwards compatibility when no file attachments require the OTSMeta format.

```console
# Encrypt using OpenSSL AES-256-GCM
# openssl enc -aes-256-gcm -pbkdf2 -md sha512 -iter 300000 -pass pass:12345678 -a -A <<<"I'm a secret"
U2FsdGVkX19G3GuIw3LGM0PVQmavPU/LnWvJhcLeYvs=

# Submit via curl API
# curl -H 'Content-Type: application/json' -d '{"secret": "U2FsdGVkX19G3GuIw3LGM0PVQmavPU/LnWvJhcLeYvs="}' http://127.0.0.1:3000/api/create
{"success":true,"expires_at":"2026-12-31T23:59:59Z","secret_id":"bbd53ec5-8ee9-4df5-a630-9561313a348a"}

# Or create & fetch natively via ots-cli:
# ots-cli create "I'm a secret"
INFO[0000] reading secret content...
INFO[0000] creating the secret...
INFO[0000] secret created, see URL below                 expires-at="2026-12-31 23:59:59 +0000 UTC"
http://127.0.0.1:3000/#bbd53ec5-8ee9-4df5-a630-9561313a348a%7C12345678901234567890

# ots-cli fetch "http://127.0.0.1:3000/#bbd53ec5-8ee9-4df5-a630-9561313a348a%7C12345678901234567890"
INFO[0000] fetching secret...
I'm a secret
```

## OTSMeta Format

The OTSMeta format was first introduced in `v1.9.0` of OTS together with the possibility to attach files to the secret. It contains structured data with a banner to differentiate between a simple JSON shared through OTS and the OTSMeta format. The OTSMeta structure itself is a simple JSON document containing a secret and a number of attachments having their contents base64 encoded:

```json
{
  "secret": "I'm a secret",
  "attachments": [
    {
      "name": "file.txt",
      "type": "text/plain",
      "data": "SSdtIGZpbGUgY29udGVudAo="
    }
  ]
}
```

This structure is prefixed with the Banner `OTSMeta` and encrypted with AES-256-GCM before being shared:

```console
# ots-cli create -f file.txt -n "I'm a secret"
INFO[0000] reading secret content...
INFO[0000] attaching file...                             file=file.txt
INFO[0000] creating the secret...
INFO[0000] secret created, see URL below                 expires-at="2026-12-31 23:59:59 +0000 UTC"
http://127.0.0.1:3000/#6a6be08c-97d7-4970-a202-5bb6964460d8%7CwNUURZ0LRrQAhaczdZfj

# Fetch via API curl & decrypt via OpenSSL AES-256-GCM:
# curl -sS http://127.0.0.1:3000/api/get/6a6be08c-97d7-4970-a202-5bb6964460d8 | jq -r .secret >/tmp/secret.bin
# openssl enc -aes-256-gcm -pbkdf2 -md sha512 -iter 300000 -pass pass:wNUURZ0LRrQAhaczdZfj -a -A -d </tmp/secret.bin
OTSMeta{"secret":"I'm a secret\n","attachments":[{"name":"file.txt","type":"text/plain; charset=utf-8","data":"SSdtIGZpbGUgY29udGVudAo="}]}

# Or fetch natively via ots-cli:
# ots-cli fetch "http://127.0.0.1:3000/#6a6be08c-97d7-4970-a202-5bb6964460d8%7CwNUURZ0LRrQAhaczdZfj"
INFO[0000] fetching secret...
I'm a secret
Attachment downloaded: file.txt
```

Of course it's also possible to share a simple secret in OTSMeta format but the recommended way would be to omit the OTSMeta wrapping:
```
OTSMeta{"secret":"I'm a secret"}
```

When programmatically reading secrets you therefore need to check whether the secret starts with `OTSMeta` and decode the remaining as a JSON document and if it does not just use all the content as the secret.

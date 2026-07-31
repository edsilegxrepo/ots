package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOTSMetaSerializationAndDeserialization(t *testing.T) {
	passphrase := "MySecurePass123"

	// 1. Plain Secret (no attachments)
	plain := Secret{Secret: "Simple text secret"}
	encPlain, err := plain.serialize(passphrase)
	require.NoError(t, err)
	assert.NotEmpty(t, encPlain)

	var decPlain Secret
	err = decPlain.read(encPlain, passphrase)
	require.NoError(t, err)
	assert.Equal(t, "Simple text secret", decPlain.Secret)
	assert.Empty(t, decPlain.Attachments)

	// 2. Secret with Attachments
	withFiles := Secret{
		Secret: "Secret with attachments",
		Attachments: []SecretAttachment{
			{Name: "hello.txt", Type: "text/plain", Content: []byte("Hello World!")},
			{Name: "data.bin", Type: "application/octet-stream", Content: []byte{0x01, 0x02, 0x03, 0x04}},
		},
	}

	encFiles, err := withFiles.serialize(passphrase)
	require.NoError(t, err)
	assert.NotEmpty(t, encFiles)

	var decFiles Secret
	err = decFiles.read(encFiles, passphrase)
	require.NoError(t, err)
	assert.Equal(t, "Secret with attachments", decFiles.Secret)
	require.Len(t, decFiles.Attachments, 2)
	assert.Equal(t, "hello.txt", decFiles.Attachments[0].Name)
	assert.Equal(t, []byte("Hello World!"), decFiles.Attachments[0].Content)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, decFiles.Attachments[1].Content)

	// 3. Wrong Passphrase Decryption Error
	var decWrongPass Secret
	err = decWrongPass.read(encFiles, "WrongPassphrase")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decrypting data")

	// 4. Corrupted Attachment Base64 Error
	corruptedJSON := []byte(`OTSMeta{"secret":"test","attachments":[{"name":"bad.txt","type":"text/plain","data":"!!!not_base64!!!"}]}`)
	encCorrupted, err := plain.serialize(passphrase)
	require.NoError(t, err)
	_ = encCorrupted // generate valid structure to test read wrapper

	var decCorrupted Secret
	err = decCorrupted.read(corruptedJSON, "") // unencrypted call with bad base64
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding attachment 0")

	// 5. Malformed OTSMeta JSON Error
	malformedJSON := []byte(`OTSMeta{invalid_json_here}`)
	var decMalformed Secret
	err = decMalformed.read(malformedJSON, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decoding JSON payload")
}

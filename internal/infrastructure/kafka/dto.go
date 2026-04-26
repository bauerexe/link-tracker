package kafka

import (
	"fmt"

	"github.com/hamba/avro/v2"
)

type UpdateLinkAvro struct {
	ID          int64   `avro:"id"`
	URL         string  `avro:"url"`
	Description string  `avro:"description"`
	TgChatIDs   []int64 `avro:"tgChatIds"`
}

type AvroCodec struct {
	schema avro.Schema
}

func NewAvroCodec(schemaText string) (*AvroCodec, error) {
	schema, err := avro.Parse(schemaText)
	if err != nil {
		return nil, fmt.Errorf("error parsing avro schema: %w", err)
	}

	return &AvroCodec{schema: schema}, nil
}

func (c *AvroCodec) Marshal(msg UpdateLinkAvro) ([]byte, error) {
	bytes, err := avro.Marshal(c.schema, msg)
	if err != nil {
		return nil, fmt.Errorf("error marshalling avro schema: %w", err)
	}
	return bytes, nil
}

func (c *AvroCodec) Unmarshal(data []byte, msg *UpdateLinkAvro) error {
	err := avro.Unmarshal(c.schema, data, msg)
	if err != nil {
		return fmt.Errorf("error unmarshalling avro schema: %w", err)
	}
	return nil
}

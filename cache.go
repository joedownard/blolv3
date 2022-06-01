package main

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	client        *dynamodb.Client
	TABLE_ENV_KEY = "BOT_DYNAMODB_TABLE_NAME"
)

type MemberRoles struct {
	UserId  string
	GuildId string
	RoleIds []string
}

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO(), func(o *config.LoadOptions) error {
		o.Region = "eu-west-2"
		return nil
	})
	if err != nil {
		panic(err)
	}
	client = dynamodb.NewFromConfig(cfg)
}

func getMemberRolesFromCache(userId string, guildId string) (MemberRoles, error) {
	params := dynamodb.GetItemInput{
		Key: map[string]types.AttributeValue{
			"UserId": &types.AttributeValueMemberS{
				Value: userId,
			},
			"GuildId": &types.AttributeValueMemberS{
				Value: guildId,
			},
		},
		TableName: aws.String(os.Getenv(TABLE_ENV_KEY)),
	}

	result, err := client.GetItem(context.TODO(), &params)
	if err != nil {
		log.Println("Error getting item,", err)
		return MemberRoles{}, err
	}

	if result.Item == nil {
		log.Println("No roles saved for this user yet,", err)
		return MemberRoles{}, err
	}

	userRoles := MemberRoles{}

	err = attributevalue.UnmarshalMap(result.Item, &userRoles)
	if err != nil {
		log.Println("Error unmarshaling item,", err)
		return MemberRoles{}, err
	}

	return userRoles, nil
}

func saveMemberRolesToCache(memberRoles MemberRoles) error {
	item, err := attributevalue.MarshalMap(memberRoles)
	if err != nil {
		log.Println("Error marshaling item,", err)
		return err
	}

	params := dynamodb.PutItemInput{
		TableName: aws.String(os.Getenv(TABLE_ENV_KEY)),
		Item:      item,
	}

	_, err = client.PutItem(context.TODO(), &params)

	if err != nil {
		log.Println("Error putting item,", err)
	}

	return err
}

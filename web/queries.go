package web

import "github.com/gschier/schier.dev/generated/prisma-client"

func RecentBlogPosts(limit int32) * prisma.BlogPostsParams {
	blogPostsOrderBy := prisma.BlogPostOrderByInputDateDesc
	return &prisma.BlogPostsParams{
		Where: &prisma.BlogPostWhereInput{
			Published: prisma.Bool(true),
			DateGt:    prisma.Str("2017-01-01"),
		},
		First:   prisma.Int32(limit),
		OrderBy: &blogPostsOrderBy,
	}
}
package muffet

type checkerOptions struct {
	fetcherOptions
	FollowRobotsTxt,
	FollowSitemapXML,
	SkipTLSVerification bool
}

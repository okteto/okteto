package graphql

import (
	"fmt"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/okteto/app/api/app"
	"github.com/okteto/app/api/github"
	"github.com/okteto/app/api/k8s/namespaces"
	"github.com/okteto/app/api/k8s/serviceaccounts"
	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"
	"github.com/opentracing/opentracing-go"
)

type credential struct {
	Config string
}

var spaceType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Space",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.ID,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"members": &graphql.Field{
				Type: graphql.NewList(memberType),
			},
		},
	},
)

var memberType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "member",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.ID,
			},
			"githubID": &graphql.Field{
				Type: graphql.String,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"avatar": &graphql.Field{
				Type: graphql.String,
			},
			"owner": &graphql.Field{
				Type: graphql.Boolean,
			},
		},
	},
)

var devEnvironmentType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "DevEnvironment",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.ID,
			},
			"space": &graphql.Field{
				Type: graphql.String,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"dev": &graphql.Field{
				Type: memberType,
			},
			"endpoints": &graphql.Field{
				Type: graphql.NewList(graphql.String),
			},
		},
	},
)

var credentialsType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Credential",
		Fields: graphql.Fields{
			"config": &graphql.Field{
				Type: graphql.String,
			},
		},
	},
)

var authenticatedUserType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "me",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.ID,
			},
			"githubID": &graphql.Field{
				Type: graphql.String,
			},
			"email": &graphql.Field{
				Type: graphql.String,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"token": &graphql.Field{
				Type: graphql.String,
			},
			"avatar": &graphql.Field{
				Type: graphql.String,
			},
		},
	},
)

var databaseType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Database",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.ID,
			},
			"space": &graphql.Field{
				Type: graphql.String,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"endpoint": &graphql.Field{
				Type: graphql.String,
			},
		},
	},
)

var queryType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"environments": &graphql.Field{
				Type:        graphql.NewList(devEnvironmentType),
				Description: "Get environment list",
				Args: graphql.FieldConfigArgument{
					"space": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					return func() (interface{}, error) {
						span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.environments")
						defer span.Finish()

						u, err := validateToken(ctx)
						if err != nil {
							return nil, err
						}
						space := u.ID
						if params.Args["space"] != nil {
							space = params.Args["space"].(string)
						}
						s, err := namespaces.GetSpaceByID(ctx, space, u)
						if err != nil {
							return nil, err
						}

						l, err := app.ListDevEnvs(ctx, u, s)
						if err != nil {
							log.Errorf("failed to get dev envs for %s in %s", u.ID, s.ID)
							return nil, fmt.Errorf("failed to get your environments")
						}

						return l, nil
					}, nil
				},
			},
			"space": &graphql.Field{
				Type:        spaceType,
				Description: "Get space",
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					return func() (interface{}, error) {
						span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.space")
						defer span.Finish()

						u, err := validateToken(ctx)
						if err != nil {
							return nil, err
						}
						id := params.Args["id"].(string)
						s, err := namespaces.GetSpaceByID(ctx, id, u)
						if err != nil {
							return nil, err
						}
						return s, nil
					}, nil
				},
			},
			"databases": &graphql.Field{
				Type:        graphql.NewList(databaseType),
				Description: "Get databases of the space",
				Args: graphql.FieldConfigArgument{
					"space": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					return func() (interface{}, error) {
						span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.databases")
						defer span.Finish()
						u, err := validateToken(ctx)
						if err != nil {
							return nil, err
						}
						space := u.ID
						if params.Args["space"] != nil {
							space = params.Args["space"].(string)
						}
						s, err := namespaces.GetSpaceByID(ctx, space, u)
						if err != nil {
							return nil, err
						}

						l, err := app.ListDatabases(s)
						if err != nil {
							log.Errorf("failed to get databases for %s in %s", u.ID, s.ID)
							return nil, fmt.Errorf("failed to get your databases")
						}

						return l, nil
					}, nil
				},
			},
			"spaces": &graphql.Field{
				Type:        graphql.NewList(spaceType),
				Description: "Get space list",
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.spaces")
					defer span.Finish()
					u, err := validateToken(ctx)
					if err != nil {
						return nil, err
					}

					l, err := app.ListSpaces(ctx, u)
					if err != nil {
						log.Errorf("failed to get spaces for %s: %s", u.ID, err)
						return nil, fmt.Errorf("failed to get your spaces")
					}

					return l, nil
				},
			},
			"credentials": &graphql.Field{
				Type:        credentialsType,
				Description: "Get credentials of the space",
				Args: graphql.FieldConfigArgument{
					"space": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					u, err := validateToken(params.Context)
					if err != nil {
						return nil, err
					}

					space := u.ID
					if params.Args["space"] != nil {
						space = params.Args["space"].(string)
					}
					c, err := app.GetCredential(u, space)
					if err != nil {
						log.Errorf("failed to get credentials: %s", err)
						return nil, fmt.Errorf("failed to get credentials")
					}

					return credential{Config: c}, nil
				},
			},
		},
	})

var mutationType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"auth": &graphql.Field{
				Type:        authenticatedUserType,
				Description: "Authenticate a user with github",
				Args: graphql.FieldConfigArgument{
					"code": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					code := params.Args["code"].(string)
					u, err := github.Auth(params.Context, code)
					if err != nil {
						log.Errorf("failed to auth user: %s", err)
						return nil, fmt.Errorf("failed to authenticate")
					}

					if err := app.CreateUser(u); err != nil {
						log.Errorf("failed to create user for %s: %s", u.ID, err)
						return nil, fmt.Errorf("failed to create your user")
					}

					return u, nil
				},
			},
			"up": &graphql.Field{
				Type:        devEnvironmentType,
				Description: "Create dev mode",
				Args: graphql.FieldConfigArgument{
					"space": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"name": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"image": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"workdir": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"devPath": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"volumes": &graphql.ArgumentConfig{
						Type: graphql.NewList(graphql.String),
					},
					"attach": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.Boolean),
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.up")
					defer span.Finish()

					u, err := validateToken(ctx)
					if err != nil {
						return nil, err
					}
					space := u.ID
					if params.Args["space"] != nil {
						space = params.Args["space"].(string)
					}
					s, err := namespaces.GetSpaceByID(ctx, space, u)
					if err != nil {
						return nil, err
					}

					dev := buildDev(params.Args)
					if err := app.DevModeOn(dev, s); err != nil {
						log.Errorf("failed to enable dev mode: %s", err)
						return nil, fmt.Errorf("failed to enable dev mode")
					}

					dev.Endpoints = app.BuildEndpoints(dev, s)
					dev.Space = space
					return dev, nil

				},
			},
			"down": &graphql.Field{
				Type:        devEnvironmentType,
				Description: "Delete dev space",
				Args: graphql.FieldConfigArgument{
					"space": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"name": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.down")
					defer span.Finish()
					u, err := validateToken(ctx)
					if err != nil {
						return nil, err
					}

					space := u.ID
					if params.Args["space"] != nil {
						space = params.Args["space"].(string)
					}
					s, err := namespaces.GetSpaceByID(ctx, space, u)
					if err != nil {
						return nil, err
					}
					dev := &model.Dev{
						Name: params.Args["name"].(string),
					}
					if err := app.DevModeOff(dev, s); err != nil {
						log.Errorf("failed to enable dev mode: %s", err)
						return nil, fmt.Errorf("failed to enable dev mode")
					}

					dev.Space = space
					return dev, nil

				},
			},
			"run": &graphql.Field{
				Type:        devEnvironmentType,
				Description: "Run a docker image",
				Args: graphql.FieldConfigArgument{
					"space": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"name": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"image": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.run")
					defer span.Finish()
					u, err := validateToken(ctx)
					if err != nil {
						return nil, err
					}
					space := u.ID
					if params.Args["space"] != nil {
						space = params.Args["space"].(string)
					}
					s, err := namespaces.GetSpaceByID(ctx, space, u)
					if err != nil {
						return nil, err
					}
					dev := &model.Dev{
						Name:  params.Args["name"].(string),
						Image: params.Args["image"].(string),
					}
					if err := app.RunImage(dev, s); err != nil {
						log.Errorf("failed to run image: %s", err)
						return nil, fmt.Errorf("failed to run image")
					}

					dev.Endpoints = app.BuildEndpoints(dev, s)
					dev.Space = space
					return dev, nil
				},
			},
			"createDatabase": &graphql.Field{
				Type:        databaseType,
				Description: "Create a database",
				Args: graphql.FieldConfigArgument{
					"space": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"name": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.createdatabase")
					defer span.Finish()
					u, err := validateToken(ctx)
					if err != nil {
						return nil, err
					}
					space := u.ID
					if params.Args["space"] != nil {
						space = params.Args["space"].(string)
					}
					s, err := namespaces.GetSpaceByID(ctx, space, u)
					if err != nil {
						return nil, err
					}
					db := &model.DB{
						Name: params.Args["name"].(string),
					}
					err = app.CreateDatabase(db, s)
					if err != nil {
						log.Errorf("failed to create database for %s: %s", u.ID, err)
						return nil, fmt.Errorf("failed to create your database")
					}

					db.Endpoint = db.GetEndpoint()
					db.Space = space
					return db, nil
				},
			},
			"deleteDatabase": &graphql.Field{
				Type:        databaseType,
				Description: "Delete a database",
				Args: graphql.FieldConfigArgument{
					"space": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"name": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.deletedatbase")
					defer span.Finish()
					u, err := validateToken(ctx)
					if err != nil {
						return nil, err
					}
					space := u.ID
					if params.Args["space"] != nil {
						space = params.Args["space"].(string)
					}
					s, err := namespaces.GetSpaceByID(ctx, space, u)
					if err != nil {
						return nil, err
					}
					db := &model.DB{
						Name: params.Args["name"].(string),
					}
					err = app.DestroyDatabase(db, s)
					if err != nil {
						log.Errorf("failed to destroy database for %s: %s", u.ID, err)
						return nil, fmt.Errorf("failed to delete your database")
					}
					db.Space = space
					return db, nil
				},
			},
			"createSpace": &graphql.Field{
				Type:        spaceType,
				Description: "Create a space",
				Args: graphql.FieldConfigArgument{
					"name": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"members": &graphql.ArgumentConfig{
						Type: graphql.NewList(graphql.String),
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.createspace")
					defer span.Finish()
					u, err := validateToken(ctx)
					if err != nil {
						return nil, err
					}
					name := params.Args["name"].(string)
					if app.ExistsByName(ctx, name, u.ID) {
						return nil, fmt.Errorf("'%s' already exists", name)
					}
					members := []model.Member{}
					if params.Args["members"] != nil {
						for _, m := range params.Args["members"].([]interface{}) {
							uMember, err := serviceaccounts.GetUserByGithubID(ctx, m.(string))
							if err != nil {
								return nil, err
							}
							members = append(
								members,
								model.Member{
									ID:       uMember.ID,
									Name:     uMember.Name,
									GithubID: uMember.GithubID,
									Avatar:   uMember.Avatar,
									Owner:    false,
								},
							)
						}
					}
					s := model.NewSpace(name, u, members)
					err = app.CreateSpace(s)
					if err != nil {
						log.Errorf("failed to create space for %s: %s", u.ID, err)
						return nil, fmt.Errorf("failed to create space")
					}
					return s, nil
				},
			},
			"updateSpace": &graphql.Field{
				Type:        spaceType,
				Description: "Update a space",
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
					"members": &graphql.ArgumentConfig{
						Type: graphql.NewList(graphql.String),
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.updatespace")
					defer span.Finish()
					u, err := validateToken(ctx)
					if err != nil {
						return nil, err
					}
					space := u.ID
					if params.Args["id"] != nil {
						space = params.Args["id"].(string)
					}
					s, err := namespaces.GetSpaceByID(ctx, space, u)
					if err != nil {
						return nil, err
					}
					if !u.IsOwner(s) {
						return nil, fmt.Errorf("your are not the owner of the space")
					}
					members := []model.Member{}
					if params.Args["members"] != nil {
						for _, m := range params.Args["members"].([]interface{}) {
							uMember, err := serviceaccounts.GetUserByGithubID(ctx, m.(string))
							if err != nil {
								return nil, err
							}
							members = append(
								members,
								model.Member{
									ID:       uMember.ID,
									Name:     uMember.Name,
									GithubID: uMember.GithubID,
									Avatar:   uMember.Avatar,
									Owner:    uMember.ID == u.ID,
								},
							)
						}
					}
					s.Members = members
					err = app.CreateSpace(s)
					if err != nil {
						log.Errorf("failed to update space for %s: %s", u.ID, err)
						return nil, fmt.Errorf("failed to update space")
					}
					return s, nil
				},
			},
			"leaveSpace": &graphql.Field{
				Type:        spaceType,
				Description: "Leave a space",
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.leavespace")
					defer span.Finish()
					u, err := validateToken(ctx)
					if err != nil {
						return nil, err
					}
					space := u.ID
					if params.Args["id"] != nil {
						space = params.Args["id"].(string)
					}
					s, err := namespaces.GetSpaceByID(ctx, space, u)
					if err != nil {
						return nil, err
					}
					if u.IsOwner(s) {
						return nil, fmt.Errorf("the owner of the space cannot leave it")
					}
					members := []model.Member{}
					for _, m := range s.Members {
						if m.ID != u.ID {
							members = append(members, m)
						}
					}
					s.Members = members

					err = app.CreateSpace(s)
					if err != nil {
						log.Errorf("failed to update space for %s: %s", u.ID, err)
						return nil, fmt.Errorf("failed to update space")
					}
					return s, nil
				},
			},
			"deleteSpace": &graphql.Field{
				Type:        spaceType,
				Description: "Delete a space",
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					span, ctx := opentracing.StartSpanFromContext(params.Context, "graphql.deletespace")
					defer span.Finish()
					u, err := validateToken(ctx)
					if err != nil {
						return nil, err
					}
					space := u.ID
					if params.Args["id"] != nil {
						space = params.Args["id"].(string)
					}
					s, err := namespaces.GetSpaceByID(ctx, space, u)
					if err != nil {
						return nil, err
					}
					if !u.IsOwner(s) {
						return nil, fmt.Errorf("your are not the owner of the space")
					}
					if space == u.ID {
						return nil, fmt.Errorf("the personal namespace cannot be deleted")
					}
					err = app.DeleteSpace(s)
					if err != nil {
						log.Errorf("failed to delete space for %s: %s", u.ID, err)
						return nil, fmt.Errorf("failed to delete space")
					}
					return s, nil
				},
			},
		},
	},
)

func buildDev(args map[string]interface{}) *model.Dev {
	dev := &model.Dev{
		Name:    strings.ToLower(args["name"].(string)),
		Image:   args["image"].(string),
		WorkDir: args["workdir"].(string),
		DevPath: args["devPath"].(string),
		Volumes: []string{},
		Attach:  args["attach"].(bool),
	}
	if args["volumes"] != nil {
		for _, v := range args["volumes"].([]interface{}) {
			dev.Volumes = append(dev.Volumes, v.(string))
		}
	}
	return dev
}

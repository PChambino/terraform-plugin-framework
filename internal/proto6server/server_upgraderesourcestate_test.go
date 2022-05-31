package proto6server

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/internal/fwserver"
	"github.com/hashicorp/terraform-plugin-framework/internal/testing/testprovider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestServerUpgradeResourceState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schema := tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"optional_attribute": {
				Type:     types.StringType,
				Optional: true,
			},
			"required_attribute": {
				Type:     types.StringType,
				Required: true,
			},
		},
		Version: 1, // Must be above 0
	}
	schemaType := schema.TerraformType(ctx)

	testCases := map[string]struct {
		server           *Server
		request          *tfprotov6.UpgradeResourceStateRequest
		expectedResponse *tfprotov6.UpgradeResourceStateResponse
		expectedError    error
	}{
		"nil": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{},
				},
			},
			request:          nil,
			expectedResponse: &tfprotov6.UpgradeResourceStateResponse{},
		},
		"request-RawState": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{
						GetResourcesMethod: func(_ context.Context) (map[string]tfsdk.ResourceType, diag.Diagnostics) {
							return map[string]tfsdk.ResourceType{
								"test_resource": &testprovider.ResourceType{
									GetSchemaMethod: func(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
										return schema, nil
									},
									NewResourceMethod: func(_ context.Context, _ tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
										return &testprovider.ResourceWithUpgradeState{
											Resource: &testprovider.Resource{},
											UpgradeStateMethod: func(ctx context.Context) map[int64]tfsdk.ResourceStateUpgrader {
												return map[int64]tfsdk.ResourceStateUpgrader{
													0: {
														StateUpgrader: func(_ context.Context, req tfsdk.UpgradeResourceStateRequest, resp *tfsdk.UpgradeResourceStateResponse) {
															expectedRawState := testNewRawState(t, map[string]interface{}{
																"id":                 "test-id-value",
																"required_attribute": true,
															})

															if diff := cmp.Diff(req.RawState, expectedRawState); diff != "" {
																resp.Diagnostics.AddError("unexpected req.RawState difference: %s", diff)
															}

															// Prevent Missing Upgraded Resource State error
															resp.State = tfsdk.State{
																Raw: tftypes.NewValue(schemaType, map[string]tftypes.Value{
																	"id":                 tftypes.NewValue(tftypes.String, "test-id-value"),
																	"optional_attribute": tftypes.NewValue(tftypes.String, nil),
																	"required_attribute": tftypes.NewValue(tftypes.String, "true"),
																}),
																Schema: schema,
															}
														},
													},
												}
											},
										}, nil
									},
								},
							}, nil
						},
					},
				},
			},
			request: &tfprotov6.UpgradeResourceStateRequest{
				RawState: testNewRawState(t, map[string]interface{}{
					"id":                 "test-id-value",
					"required_attribute": true,
				}),
				TypeName: "test_resource",
				Version:  0,
			},
			expectedResponse: &tfprotov6.UpgradeResourceStateResponse{
				UpgradedState: testNewDynamicValue(t, schemaType, map[string]tftypes.Value{
					"id":                 tftypes.NewValue(tftypes.String, "test-id-value"),
					"optional_attribute": tftypes.NewValue(tftypes.String, nil),
					"required_attribute": tftypes.NewValue(tftypes.String, "true"),
				}),
			},
		},
		"request-TypeName-missing": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{},
				},
			},
			request: &tfprotov6.UpgradeResourceStateRequest{},
			expectedResponse: &tfprotov6.UpgradeResourceStateResponse{
				Diagnostics: []*tfprotov6.Diagnostic{
					{
						Severity: tfprotov6.DiagnosticSeverityError,
						Summary:  "Resource Type Not Found",
						Detail:   "No resource type named \"\" was found in the provider.",
					},
				},
			},
		},
		"request-TypeName-unknown": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{},
				},
			},
			request: &tfprotov6.UpgradeResourceStateRequest{
				TypeName: "unknown",
			},
			expectedResponse: &tfprotov6.UpgradeResourceStateResponse{
				Diagnostics: []*tfprotov6.Diagnostic{
					{
						Severity: tfprotov6.DiagnosticSeverityError,
						Summary:  "Resource Type Not Found",
						Detail:   "No resource type named \"unknown\" was found in the provider.",
					},
				},
			},
		},
		"response-Diagnostics": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{
						GetResourcesMethod: func(_ context.Context) (map[string]tfsdk.ResourceType, diag.Diagnostics) {
							return map[string]tfsdk.ResourceType{
								"test_resource": &testprovider.ResourceType{
									GetSchemaMethod: func(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
										return schema, nil
									},
									NewResourceMethod: func(_ context.Context, _ tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
										return &testprovider.ResourceWithUpgradeState{
											Resource: &testprovider.Resource{},
											UpgradeStateMethod: func(ctx context.Context) map[int64]tfsdk.ResourceStateUpgrader {
												return map[int64]tfsdk.ResourceStateUpgrader{
													0: {
														StateUpgrader: func(_ context.Context, _ tfsdk.UpgradeResourceStateRequest, resp *tfsdk.UpgradeResourceStateResponse) {
															resp.Diagnostics.AddWarning("warning summary", "warning detail")
															resp.Diagnostics.AddError("error summary", "error detail")
														},
													},
												}
											},
										}, nil
									},
								},
							}, nil
						},
					},
				},
			},
			request: &tfprotov6.UpgradeResourceStateRequest{
				RawState: testNewRawState(t, map[string]interface{}{
					"id":                 "test-id-value",
					"required_attribute": true,
				}),
				TypeName: "test_resource",
				Version:  0,
			},
			expectedResponse: &tfprotov6.UpgradeResourceStateResponse{
				Diagnostics: []*tfprotov6.Diagnostic{
					{
						Severity: tfprotov6.DiagnosticSeverityWarning,
						Summary:  "warning summary",
						Detail:   "warning detail",
					},
					{
						Severity: tfprotov6.DiagnosticSeverityError,
						Summary:  "error summary",
						Detail:   "error detail",
					},
				},
			},
		},
		"response-UpgradedState": {
			server: &Server{
				FrameworkServer: fwserver.Server{
					Provider: &testprovider.Provider{
						GetResourcesMethod: func(_ context.Context) (map[string]tfsdk.ResourceType, diag.Diagnostics) {
							return map[string]tfsdk.ResourceType{
								"test_resource": &testprovider.ResourceType{
									GetSchemaMethod: func(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
										return schema, nil
									},
									NewResourceMethod: func(_ context.Context, _ tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
										return &testprovider.ResourceWithUpgradeState{
											Resource: &testprovider.Resource{},
											UpgradeStateMethod: func(ctx context.Context) map[int64]tfsdk.ResourceStateUpgrader {
												return map[int64]tfsdk.ResourceStateUpgrader{
													0: {
														StateUpgrader: func(_ context.Context, _ tfsdk.UpgradeResourceStateRequest, resp *tfsdk.UpgradeResourceStateResponse) {
															resp.State = tfsdk.State{
																Raw: tftypes.NewValue(schemaType, map[string]tftypes.Value{
																	"id":                 tftypes.NewValue(tftypes.String, "test-id-value"),
																	"optional_attribute": tftypes.NewValue(tftypes.String, nil),
																	"required_attribute": tftypes.NewValue(tftypes.String, "true"),
																}),
																Schema: schema,
															}
														},
													},
												}
											},
										}, nil
									},
								},
							}, nil
						},
					},
				},
			},
			request: &tfprotov6.UpgradeResourceStateRequest{
				RawState: testNewRawState(t, map[string]interface{}{
					"id":                 "test-id-value",
					"required_attribute": true,
				}),
				TypeName: "test_resource",
				Version:  0,
			},
			expectedResponse: &tfprotov6.UpgradeResourceStateResponse{
				UpgradedState: testNewDynamicValue(t, schemaType, map[string]tftypes.Value{
					"id":                 tftypes.NewValue(tftypes.String, "test-id-value"),
					"optional_attribute": tftypes.NewValue(tftypes.String, nil),
					"required_attribute": tftypes.NewValue(tftypes.String, "true"),
				}),
			},
		},
	}

	for name, testCase := range testCases {
		name, testCase := name, testCase

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := testCase.server.UpgradeResourceState(context.Background(), testCase.request)

			if diff := cmp.Diff(testCase.expectedError, err); diff != "" {
				t.Errorf("unexpected error difference: %s", diff)
			}

			if diff := cmp.Diff(testCase.expectedResponse, got); diff != "" {
				t.Errorf("unexpected response difference: %s", diff)
			}
		})
	}
}

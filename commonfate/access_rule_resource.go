package commonfate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/common-fate/common-fate/governance"

	cf_types "github.com/common-fate/common-fate/pkg/types"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type accessRuleModel struct {
	Name        types.String   `tfsdk:"name"`
	Approval    ApprovalModel  `tfsdk:"approval"`
	Description types.String   `tfsdk:"description"`
	Groups      []types.String `tfsdk:"groups"`
	ID          types.String   `tfsdk:"id"`
	Status      types.String   `tfsdk:"status"`
	// Version     types.String   `tfsdk:"version"`
	Target         []TargetProviderModel `tfsdk:"target"`
	Duration       types.String          `tfsdk:"duration"`
	TargetProvider types.String          `tfsdk:"target_provider_id"`
}

type TimeConstraintsModel struct {
	MaxDurationSeconds types.Int64 `tfsdk:"maxDurationSeconds"`
}

type ApprovalModel struct {
	Groups []types.String `tfsdk:"groups"`
	Users  []types.String `tfsdk:"users"`
}

type TargetProviderModel struct {
	Field types.String `tfsdk:"field"`
	Value []string     `tfsdk:"value"`
}

// AccessRuleResource is the data source implementation.
type AccessRuleResource struct {
	client *governance.ClientWithResponses
}

var (
	_ resource.Resource                = &AccessRuleResource{}
	_ resource.ResourceWithConfigure   = &AccessRuleResource{}
	_ resource.ResourceWithImportState = &AccessRuleResource{}
)

// Metadata returns the data source type name.
func (r *AccessRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_rule"
}

// Configure adds the provider configured client to the data source.
func (r *AccessRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*governance.ClientWithResponses)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// GetSchema defines the schema for the data source.
// schema is based off the governance api
func (r *AccessRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {

	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Access Aule ID",
				Computed:            true,
				// PlanModifiers: planmodifier.String{
				// 	stringplanmodifier.UseStateForUnknown(),
				// },
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the Access Rule",
			},
			"description": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Description of the Access Rule",
			},
			"status": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Status of the Access Rule",
			},
			"groups": schema.ListAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "groups with access to the Access Rule",
			},

			"target_provider_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "id of the provider",
			},
			"duration": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "duration of the rule",
			},
			"approval": schema.SingleNestedAttribute{
				Optional: true,

				Attributes: map[string]schema.Attribute{
					"groups": schema.ListAttribute{
						ElementType: types.StringType,
						Optional:    true,
					},
					"users": schema.ListAttribute{
						ElementType: types.StringType,
						Optional:    true,
					},
				},
			},
			"target": schema.ListNestedAttribute{
				Required: true,

				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "id of the provider",
						},
						"value": schema.ListAttribute{
							Required:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
		},
		MarkdownDescription: "Manages the creation of a Common Fate access rule.",
	}
}

func (r *AccessRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Unconfigured HTTP Client",
			"Expected configured HTTP client. Please report this issue to the provider developers.",
		)

		return
	}
	var data *accessRuleModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {

		resp.Diagnostics.AddError(
			"Unable to Create Resource",
			"An unexpected error occurred while parsing the resource creation response.",
		)

		return

	}

	// target := cf_types.CreateAccessRuleTarget{
	// 	ProviderId: data.Target.Provider.ID.ValueString(),
	// }
	dur, err := strconv.Atoi(data.Duration.ValueString())

	if err != nil {

		resp.Diagnostics.AddError(
			"failed to convert time to int",
			"An unexpected error occurred while parsing the resource creation response. "+
				"Please report this issue to the provider developers.\n\n"+
				"JSON Error: "+err.Error(),
		)

		return

	}
	createRequest := governance.GovCreateAccessRuleJSONRequestBody{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),

		// Target:          target,
		TimeConstraints: cf_types.TimeConstraints{MaxDurationSeconds: dur},
	}

	for _, g := range data.Groups {
		createRequest.Groups = append(createRequest.Groups, g.ValueString())
	}

	for _, g := range data.Approval.Groups {
		createRequest.Approval.Groups = append(createRequest.Approval.Groups, g.ValueString())
	}

	for _, u := range data.Approval.Users {
		createRequest.Approval.Users = append(createRequest.Approval.Users, u.ValueString())
	}

	args := make(map[string]cf_types.CreateAccessRuleTargetDetailArguments)
	for _, v := range data.Target {

		args[v.Field.ValueString()] = cf_types.CreateAccessRuleTargetDetailArguments{Values: v.Value}
	}

	createRequest.Target = cf_types.CreateAccessRuleTarget{ProviderId: data.TargetProvider.ValueString(), With: cf_types.CreateAccessRuleTarget_With{AdditionalProperties: args}}

	//create the new access model with the client
	res, err := r.client.GovCreateAccessRuleWithResponse(ctx, createRequest)

	if err != nil {

		resp.Diagnostics.AddError(
			"Unable to Create Resource",
			"An unexpected error occurred while parsing the resource creation response. "+
				"Please report this issue to the provider developers.\n\n"+
				"JSON Error: "+err.Error(),
		)

		return

	}

	if res.JSON201 == nil {

		resp.Diagnostics.AddError(
			"Unable to Create Resource",
			"An unexpected error occurred while parsing the resource creation response. "+
				"Please report this issue to the provider developers.\n\n"+
				"JSON Error: "+res.Status(),
		)

		return

	}

	// // Convert from the API data model to the Terraform data model
	// // and set any unknown attribute values.
	data.ID = types.StringValue(res.JSON201.ID)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *AccessRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError(
			"Unconfigured HTTP Client",
			"Expected configured HTTP client. Please report this issue to the provider developers.",
		)

		return
	}
	var state accessRuleModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	//read access rule

	accessRule, err := r.client.GovGetAccessRuleWithResponse(ctx, state.ID.ValueString())

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read access rule",
			err.Error(),
		)
		return
	}

	// Treat HTTP 404 Not Found status as a signal to recreate resource
	// and return early
	if accessRule.HTTPResponse.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError(
			"Unable to Refresh Resource",
			"An unexpected error occurred while attempting to refresh resource state. ")
		return
	}

	// Treat HTTP 404 Not Found status as a signal to recreate resource
	// and return early
	if accessRule.HTTPResponse.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)

		return
	}

	var res cf_types.AccessRuleDetail

	err = json.Unmarshal(accessRule.Body, &res)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Refresh Resource",
			"An unexpected error occurred while attempting to refresh resource state. "+
				"Please retry the operation or report this issue to the provider developers.\n\n"+
				"HTTP Error: "+err.Error(),
		)
		return
	}

	// Convert from the API data model to the Terraform data model
	// and refresh any attribute values.
	state.Name = types.StringValue(res.Name)
	state.Description = types.StringValue(res.Description)
	dur := strconv.Itoa(res.TimeConstraints.MaxDurationSeconds)
	state.Duration = types.StringValue(dur)

	for _, g := range res.Groups {
		state.Groups = append(state.Groups, types.StringValue(g))

	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AccessRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError(
			"Unconfigured HTTP Client",
			"Expected configured HTTP client. Please report this issue to the provider developers.",
		)
	}
	var data *accessRuleModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {

		resp.Diagnostics.AddError(
			"Unable to Create Resource",
			"An unexpected error occurred while parsing the resource creation response.",
		)

		return

	}

	// target := cf_types.CreateAccessRuleTarget{
	// 	ProviderId: data.Target.Provider.ID.ValueString(),
	// }

	// //create the new access model with the client
	// _, err := r.client.GovUpdateAccessRuleWithResponse(ctx, data.ID.ValueString(), governance.GovUpdateAccessRuleJSONRequestBody{
	// 	Name:        data.Name.ValueString(),
	// 	Description: data.Description.ValueString(),
	// 	Target:      target,
	// })

	// if err != nil {

	// 	resp.Diagnostics.AddError(
	// 		"Unable to Create Resource",
	// 		"An unexpected error occurred while parsing the resource creation response. "+
	// 			"Please report this issue to the provider developers.\n\n"+
	// 			"JSON Error: "+err.Error(),
	// 	)

	// 	return

	// }

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

func (r *AccessRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError(
			"Unconfigured HTTP Client",
			"Expected configured HTTP client. Please report this issue to the provider developers.",
		)
	}
	var data *accessRuleModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {

		resp.Diagnostics.AddError(
			"Unable to Create Resource",
			"An unexpected error occurred while parsing the resource creation response.",
		)

		return

	}

	// //create the new access model with the client
	// _, err := r.client.GovArchiveAccessRuleWithResponse(ctx, data.ID.ValueString())
	// if err != nil {

	// 	resp.Diagnostics.AddError(
	// 		"Unable to Create Resource",
	// 		"An unexpected error occurred while parsing the resource creation response. "+
	// 			"Please report this issue to the provider developers.\n\n"+
	// 			"JSON Error: "+err.Error(),
	// 	)

	// 	return

	// }

}

func (r *AccessRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const metaAPIVersion = "v21.0"
const metaAPIBase = "https://graph.facebook.com"

type MetaClient struct {
	AccessToken string
}

func NewMetaClient(accessToken string) *MetaClient {
	return &MetaClient{AccessToken: accessToken}
}

type MetaError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    int    `json:"code"`
		Subcode int    `json:"error_subcode"`
	} `json:"error"`
}

func (e *MetaError) IsRateLimit() bool {
	return e.Error.Code == 17 || e.Error.Code == 4
}

func (e *MetaError) IsTokenInvalid() bool {
	return e.Error.Code == 190
}

func (c *MetaClient) get(path string, params url.Values) ([]byte, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("access_token", c.AccessToken)

	reqURL := fmt.Sprintf("%s/%s/%s?%s", metaAPIBase, metaAPIVersion, path, params.Encode())

	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var metaErr MetaError
	if err := json.Unmarshal(body, &metaErr); err == nil && metaErr.Error.Code != 0 {
		return nil, fmt.Errorf("meta error %d: %s", metaErr.Error.Code, metaErr.Error.Message)
	}

	return body, nil
}

func (c *MetaClient) post(path string, data url.Values) ([]byte, error) {
	if data == nil {
		data = url.Values{}
	}
	data.Set("access_token", c.AccessToken)

	reqURL := fmt.Sprintf("%s/%s/%s", metaAPIBase, metaAPIVersion, path)
	resp, err := http.PostForm(reqURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var metaErr MetaError
	if err := json.Unmarshal(body, &metaErr); err == nil && metaErr.Error.Code != 0 {
		return nil, fmt.Errorf("meta error %d: %s", metaErr.Error.Code, metaErr.Error.Message)
	}

	return body, nil
}

// ValidateToken calls /me to check if token is valid
func (c *MetaClient) ValidateToken() (string, error) {
	body, err := c.get("me", url.Values{"fields": {"id,name"}})
	if err != nil {
		return "", err
	}
	var result struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	return result.Name, nil
}

type MetaAdAccount struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	AccountStatus int    `json:"account_status"`
	Currency      string `json:"currency"`
	TimezoneName  string `json:"timezone_name"`
}

// FetchAdAccounts retrieves all ad accounts accessible by the token
func (c *MetaClient) FetchAdAccounts() ([]MetaAdAccount, error) {
	params := url.Values{
		"fields": {"id,name,account_status,currency,timezone_name"},
		"limit":  {"100"},
	}

	body, err := c.get("me/adaccounts", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []MetaAdAccount `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

type MetaCampaign struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Objective   string `json:"objective"`
	DailyBudget string `json:"daily_budget"`
}

// FetchCampaigns retrieves campaigns for a given ad account
func (c *MetaClient) FetchCampaigns(adAccountID string) ([]MetaCampaign, error) {
	params := url.Values{
		"fields": {"id,name,status,objective,daily_budget"},
		"limit":  {"500"},
	}

	body, err := c.get(fmt.Sprintf("act_%s/campaigns", adAccountID), params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []MetaCampaign `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

type CreateCampaignReq struct {
	Name                string
	Objective           string
	SpecialAdCategories []string
	Status              string
	DailyBudget         int64
}

func (c *MetaClient) CreateCampaign(adAccountID string, req CreateCampaignReq) (string, error) {
	cats := "[]"
	if len(req.SpecialAdCategories) > 0 && req.SpecialAdCategories[0] != "NONE" {
		b, _ := json.Marshal(req.SpecialAdCategories)
		cats = string(b)
	}

	data := url.Values{
		"name":                  {req.Name},
		"objective":             {req.Objective},
		"special_ad_categories": {cats},
		"status":                {req.Status},
	}

	if req.DailyBudget > 0 {
		data.Set("daily_budget", fmt.Sprintf("%d", req.DailyBudget))
	}

	body, err := c.post(fmt.Sprintf("act_%s/campaigns", adAccountID), data)
	if err != nil {
		return "", err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.ID, nil
}

type CreateAdSetReq struct {
	Name         string
	CampaignID   string
	DailyBudget  int64
	BidStrategy  string
	TargetingJSON string
	StartTime    string
	EndTime      string
	Status       string
}

func (c *MetaClient) CreateAdSet(adAccountID string, req CreateAdSetReq) (string, error) {
	data := url.Values{
		"name":         {req.Name},
		"campaign_id":  {req.CampaignID},
		"daily_budget": {fmt.Sprintf("%d", req.DailyBudget)},
		"bid_strategy": {req.BidStrategy},
		"targeting":    {req.TargetingJSON},
		"status":       {req.Status},
	}

	if req.StartTime != "" {
		data.Set("start_time", req.StartTime)
	}
	if req.EndTime != "" {
		data.Set("end_time", req.EndTime)
	}

	body, err := c.post(fmt.Sprintf("act_%s/adsets", adAccountID), data)
	if err != nil {
		return "", err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.ID, nil
}

type CreateAdCreativeReq struct {
	Name         string
	ImageHash    string
	VideoID      string
	Format       string
	PrimaryText  string
	Headline     string
	Description  string
	CTA          string
	DestURL      string
	PageID       string
}

func (c *MetaClient) CreateAdCreative(adAccountID string, req CreateAdCreativeReq) (string, error) {
	var objectStory interface{}

	switch req.Format {
	case "image":
		objectStory = map[string]interface{}{
			"page_id": req.PageID,
			"link_data": map[string]interface{}{
				"image_hash":  req.ImageHash,
				"link":        req.DestURL,
				"message":     req.PrimaryText,
				"name":        req.Headline,
				"description": req.Description,
				"call_to_action": map[string]interface{}{
					"type": req.CTA,
					"value": map[string]string{
						"link": req.DestURL,
					},
				},
			},
		}
	case "video":
		objectStory = map[string]interface{}{
			"page_id": req.PageID,
			"video_data": map[string]interface{}{
				"video_id":    req.VideoID,
				"message":     req.PrimaryText,
				"title":       req.Headline,
				"description": req.Description,
				"call_to_action": map[string]interface{}{
					"type": req.CTA,
					"value": map[string]string{
						"link": req.DestURL,
					},
				},
			},
		}
	}

	storyJSON, _ := json.Marshal(objectStory)

	data := url.Values{
		"name":               {req.Name},
		"object_story_spec":  {string(storyJSON)},
	}

	body, err := c.post(fmt.Sprintf("act_%s/adcreatives", adAccountID), data)
	if err != nil {
		return "", err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.ID, nil
}

func (c *MetaClient) CreateAd(adAccountID, adSetID, creativeID, name, status string) (string, error) {
	creative := map[string]string{"creative_id": creativeID}
	creativeJSON, _ := json.Marshal(creative)

	data := url.Values{
		"name":     {name},
		"adset_id": {adSetID},
		"creative": {string(creativeJSON)},
		"status":   {status},
	}

	body, err := c.post(fmt.Sprintf("act_%s/ads", adAccountID), data)
	if err != nil {
		return "", err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.ID, nil
}

func (c *MetaClient) UpdateCampaignStatus(campaignID, status string) error {
	data := url.Values{"status": {status}}
	_, err := c.post(campaignID, data)
	return err
}

func (c *MetaClient) UpdateCampaignBudget(campaignID string, dailyBudget int64) error {
	data := url.Values{
		"daily_budget": {fmt.Sprintf("%d", dailyBudget)},
	}
	_, err := c.post(campaignID, data)
	return err
}

func (c *MetaClient) DeleteCampaign(campaignID string) error {
	reqURL := fmt.Sprintf("%s/%s/%s?access_token=%s",
		metaAPIBase, metaAPIVersion, campaignID, c.AccessToken)

	req, err := http.NewRequest(http.MethodDelete, reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete campaign failed: %s", string(body))
	}

	return nil
}

// BuildTargetingJSON creates targeting spec JSON from campaign wizard inputs
func BuildTargetingJSON(countries []string, ageMin, ageMax int, genders []int, placements []string) string {
	targeting := map[string]interface{}{
		"geo_locations": map[string]interface{}{
			"countries": countries,
		},
		"age_min": ageMin,
		"age_max": ageMax,
	}

	if len(genders) > 0 {
		targeting["genders"] = genders
	}

	if len(placements) > 0 {
		publisher := map[string][]string{}
		for _, p := range placements {
			switch {
			case strings.HasPrefix(p, "facebook_"):
				pos := strings.TrimPrefix(p, "facebook_")
				publisher["facebook"] = append(publisher["facebook"], pos)
			case strings.HasPrefix(p, "instagram_"):
				pos := strings.TrimPrefix(p, "instagram_")
				publisher["instagram"] = append(publisher["instagram"], pos)
			case p == "audience_network":
				publisher["audience_network"] = append(publisher["audience_network"], "classic")
			}
		}
		if len(publisher) > 0 {
			targeting["publisher_platforms"] = getKeys(publisher)
			for k, v := range publisher {
				targeting[k+"_positions"] = v
			}
		}
	}

	b, _ := json.Marshal(targeting)
	return string(b)
}

func getKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

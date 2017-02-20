package fbscheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ponzu-cms/ponzu/management/editor"
	"github.com/ponzu-cms/ponzu/system/addon"
	"github.com/ponzu-cms/ponzu/system/admin"
	"github.com/ponzu-cms/ponzu/system/db"
)

var meta = addon.Meta{
	PonzuAddonName:      "Facebook Scheduler",
	PonzuAddonAuthor:    "Boss Sauce Creative, LLC",
	PonzuAddonAuthorURL: "https://bosssauce.it",
	PonzuAddonVersion:   "0.1",
}

var _ = addon.Register(meta, func() interface{} { return new(PostScheduler) })

// Data exports access to the addon PostScheduler type data
var Data *PostScheduler

func init() {
	var err error
	Data, err = data()
	if err != nil {

	}
}

const (
	// FBAPI is the base URL for Facebook Graph API calls
	FBAPI = "https://graph.facebook.com"

	// FBVIDEOAPI is the base URL for uploading video via the Facebook Graph API
	FBVIDEOAPI = "https://graph-video.facebook.com"

	// FBAPIVERSION is the version of the Facebook API to use
	FBAPIVERSION = "v2.8"

	// NoAttachment defines attachment type of nothing
	NoAttachment = 1 + iota
	// VideoAttachment defines attachment type of video
	VideoAttachment
	// PhotoAttachment defines attachment type of photo
	PhotoAttachment
	// LinkAttachment defines attachment type of link
	LinkAttachment
)

// PostScheduler controls how content is scheduled to be posted to Facebook
type PostScheduler struct {
	addon.Addon

	FacebookAppID             string `json:"facebook_app_id"`
	FacebookAppSecret         string `json:"facebook_app_secret"`
	FacebookExtendedUserToken string `json:"facebook_extended_user_token"`
	FacebookPageAuthToken     string `json:"facebook_page_auth_token"`
	FacebookPageID            string `json:"facebook_page_id"`
	FacebookPageName          string `json:"facebook_page_name"`
}

type Post struct {
	Title                string   `json:"title"`
	Message              string   `json:"message"`
	Attachment           string   `json:"attachment"`      // URL, depending on AttachmentType
	AttachmentType       int      `json:"attachment_type"` // type of attachement
	Place                string   `json:"place"`           // Page ID of location associated with post (required to use tags)
	Tags                 []string `json:"tags"`            // convert to comma separated values of user IDs ex: '1234,4566,6788'
	Published            bool     `json:"published"`
	ScheduledPublishTime int64    `json:"scheduled_publish_time"` // Unix time (10 digit timestamp)
}

// MarshalEditor ...
func (p *PostScheduler) MarshalEditor() ([]byte, error) {
	fbLogin := []byte(`
	<div class="input-field col s12">
		<label class="active">Manage Facebook Connection</label>
		<div id="fb-root"></div>
		<script>
			(function(d, s, id){
				var js, fjs = d.getElementsByTagName(s)[0];
				if (d.getElementById(id)) {return;}
				js = d.createElement(s); js.id = id;
				js.src = "//connect.facebook.net/en_US/sdk.js";
				fjs.parentNode.insertBefore(js, fjs);
			}(document, 'script', 'facebook-jssdk'));
			
			var makeOption = function(page) {
				console.log(page);
				return $('<option>')
					.val(page.id)
					.text(page.name)
					.attr('data-token', page.access_token);
			}

			// showPages should display the select dropdown
			var showPages = function() {
				var selected = "` + p.FacebookPageID + `";
				var pages = [];
				var $select = $('select[name=facebook_page_id]');
				var extendedToken = $('input[name=facebook_extended_user_token]').val();

				if ($select) {
					$select.show();

					$select.on('change', function(e) {
						var option = $(e.target).find('option:selected'),
							nameStore = $('input[name=facebook_page_name]'),
							idStore = $('input[name=facebook_page_id]'),
							pageTokenStore = $('input[name=facebook_page_auth_token]');
					
						nameStore.val(option.text());

						if (idStore) {
							idStore.val(option.val());
						}

						if (pageTokenStore) {
							pageTokenStore.val(option.attr('data-token'));
						}
					});
					
					FB.api("/me/accounts?access_token="+extendedToken, function(response) {
						if (response.data.length > 0) {
							// remove existing or placeholder pages from list
							$select.empty();
						}
						
						for (var i = 0; i < response.data.length; i++) {
							var option = makeOption(response.data[i]);
							if (response.data[i].id === selected) {
								option.attr('selected', 'true');
							}
							
							pages.push(option)
						}

						$select.append(pages);	
					});
				}
			}

			var getExtendedUserAccessToken = function(tmpToken) {
				// set input value
				$('input[name=facebook_user_access_token]').val(tmpToken);

				// save form
				$('form[action="/admin/addon"]').submit();
			}

			var submitForm = function() {
				$('form[action="/admin/addon"]').submit();
			}

			window.fbAsyncInit = function() {
				FB.init({
					appId: '` + p.FacebookAppID + `',
					xfbml: true,
					version: '` + FBAPIVERSION + `'
				});

				// log user in if we don't have a page configured
				if ($('input[name=facebook_extended_user_token]').val() === "") {
					FB.login(function(response) {
						// if auth succeeds, show list of pages to choose from
						if (response.status === 'connected') {
							getExtendedUserAccessToken(response.authResponse.accessToken)
						}

					}, {
						scope: 'manage_pages,publish_pages,pages_show_list'
					});
				} else {
					showPages();
				}

				var $select = $('select[name=facebook_page_id]');
				if ($select) { $select.show(); }

				FB.Event.subscribe('auth.statusChange', FBAuthChangeHandler);
			}   		

			var FBAuthChangeHandler = function(response) {
				console.log(response);
				if (response.status !== 'connected') {
					$('a#reset').trigger('click');
				}
			}

			$(function() {
				$('a#reset').on('click', function(e) {
					e.preventDefault();
					
					// empty Facebook Page namw, id, token values, and submit form (save)
					$('input[name=facebook_page_id]').val(''); 
					$('input[name=facebook_page_name]').val('');
					$('input[name=facebook_page_auth_token]').val('')
					$('input[name=facebook_extended_user_token]').val('')

					FB.getLoginStatus(function(response) {
						if (response.status === 'connected') {
							FB.logout(function() {
								submitForm();
							})
						} else {
							submitForm();				
						}
					});

				});
			});

		</script>
		
		<div class="fb-login-button" data-max-rows="1" data-size="large" data-show-faces="false" data-auto-logout-link="true" style="margin: 20px 0px"></div>
	</div>
	`)

	// if no App ID is saved, do not show login button
	if p.FacebookAppID == "" || p.FacebookAppSecret == "" {
		fbLogin = nil
	}

	var fbPageSelect string
	var fbPageOptions string
	var activePage string

	if p.FacebookPageID == "" || p.FacebookPageName == "" {
		fbPageOptions = `<option>No Pages</option>`
	} else {
		fbPageOptions = `<option value="` + p.FacebookPageID + `">` + p.FacebookPageName + `</option>`
		resetButton := `<a href="#" id="reset" class="btn red waves-effect waves-light right">Reset</a>`
		activePageImage := `<img style="width:15px;height:15px" src="` + FBAPI + `/` + p.FacebookPageID + `/picture"/>&nbsp;`
		activePage = `<p>Active Page: ` + activePageImage + `<b>` + p.FacebookPageName + `</b> ` + resetButton + `</p>`
	}

	if p.FacebookPageID != "" && p.FacebookPageName != "" {
		fbPageSelect = `<input type="hidden" name="facebook_page_id" value="` + p.FacebookPageID + `"/>`
	} else {
		fbPageSelect = `<p>
		<select class="broswer-default" name="facebook_page_id">
		` + fbPageOptions + `
		</select>
		</p>
		<label class="active">Choose which Page to scheulde posts, then click 'Save' to active:</label>
		`
	}

	fbPagesList := []byte(`
	<div class="input-field col s12">
		` + fbPageSelect + `
		<input type="hidden" name="facebook_page_name" value="` + p.FacebookPageName + `"/>
		<input type="hidden" name="facebook_page_auth_token" value="` + p.FacebookPageAuthToken + `"/>		
		<input type="hidden" name="facebook_user_access_token" value=""/>		
		` + activePage + `
	</div>
	`)

	view, err := editor.Form(p,
		editor.Field{
			View: editor.Input("FacebookAppID", p, map[string]string{
				"label":       "Enter the App ID of your Facebook application",
				"type":        "text",
				"placeholder": "get an App ID at developers.facebook.com",
			}),
		},
		editor.Field{
			View: editor.Input("FacebookAppSecret", p, map[string]string{
				"label":       "Enter the App Secret shown in your Facebook App Settings",
				"type":        "password",
				"placeholder": "locate the App Secret at developers.facebook.com",
			}),
		},
		editor.Field{
			View: []byte(`<p>Once you have the App ID and App Secret filled in above, click 'Save' to proceed.</p>`),
		},
		editor.Field{
			View: fbLogin,
		},
		editor.Field{
			View: fbPagesList,
		},
		editor.Field{
			View: editor.Input("FacebookPageAuthToken", p, map[string]string{
				"type": "hidden",
			}),
		},
		editor.Field{
			View: editor.Input("FacebookExtendedUserToken", p, map[string]string{
				"type": "hidden",
			}),
		},
	)
	if err != nil {
		return nil, err
	}

	css := []byte(`
	<style type="text/css">
	form .card-content {
		background: #365899;
		color: #fff;
	}
	</style>
	`)

	return append(view, css...), nil
}

func data() (*PostScheduler, error) {
	var ps = &PostScheduler{}

	key, err := addon.KeyFromMeta(meta)
	if err != nil {
		return ps, err
	}
	data, err := db.Addon(key)
	if err != nil {
		return ps, err
	}

	err = json.Unmarshal(data, ps)
	if err != nil {
		return ps, err
	}

	return ps, nil
}

// Schedule sends content to Facebook to be posted at the scheduled date. In our
// case the scheduled date is the timestamp of the content
func Schedule(post Post) error {
	// get addon data
	ps, err := data()
	if err != nil {
		return err
	}
	Data = ps

	client := http.Client{Timeout: time.Second * 30}

	var fbResponse map[string]interface{}
	var edge string
	var api = FBAPI
	var params []string
	defaultParams := []string{
		"published=" + fmt.Sprintf("%t", post.Published),
		"scheduled_publish_time=" + fmt.Sprintf("%d", post.ScheduledPublishTime),
		"access_token=" + ps.FacebookPageAuthToken,
		"unpublished_content_type=SCHEDULED",
	}

	switch post.AttachmentType {
	case VideoAttachment:
		params = []string{
			"title=" + url.QueryEscape(post.Title),
			"description=" + url.QueryEscape(post.Message),
			"file_url=" + post.Attachment,
		}

		edge = "videos"
		api = FBVIDEOAPI

	case PhotoAttachment:
		id, err := strconv.ParseInt(post.Tags[0], 10, 64)
		if err != nil {
			return err
		}

		tags := fmt.Sprintf(`[{x: 0.0, y: 0.0, tag_uid: %d, tag_text: ''}]`, id)
		params = []string{
			"allow_spherical_photo=true",
			"caption=" + url.QueryEscape(post.Message),
			"url=" + post.Attachment,
			"place=" + post.Place,
			"tags=" + url.QueryEscape(tags),
		}

		edge = "photos"

	case LinkAttachment:
		params = []string{
			"message=" + url.QueryEscape(post.Message),
			"place=" + post.Place,
			"link=" + post.Attachment,
		}

		edge = "feed"

	case NoAttachment:
		params = []string{
			"message=" + url.QueryEscape(post.Message),
			"place=" + post.Place,
		}

		edge = "feed"
	}

	uri := fmt.Sprintf("%s/%s/%s/%s?", api, FBAPIVERSION, ps.FacebookPageID, edge)

	body := &bytes.Buffer{}
	params = append(defaultParams, params...)
	scheduleReq, err := http.NewRequest(http.MethodPost, uri+strings.Join(params, "&"), body)
	if err != nil {
		return err
	}

	log.Println("[fbscheduler] schedule:", scheduleReq.URL)

	resp, err := client.Do(scheduleReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, &fbResponse)
	if err != nil {
		return err
	}

	if fbResponse["error"] != nil {
		return fmt.Errorf("Error from Facebook API: %v", fbResponse["error"])
	}

	return nil
}

// BeforeSave overrides the embedded addon.Addon.Item's BeforeSave method to
// verify requirements and set up various Facebook API calls for data needed by
// the addon and its methods
func (p *PostScheduler) BeforeSave(res http.ResponseWriter, req *http.Request) error {
	// check that appID and appSecret are present
	err := req.ParseMultipartForm(1024 * 1024) // maxMemory 1MB
	if err != nil {
		v, err := admin.Error500()
		if err != nil {
			return err
		}

		res.WriteHeader(http.StatusInternalServerError)
		res.Write(v)

		return err
	}

	appID := req.FormValue("facebook_app_id")
	appSecret := req.FormValue("facebook_app_secret")
	tmpUserToken := req.FormValue("facebook_user_access_token")

	if appID == "" || appSecret == "" {
		v, err := admin.ErrorMessage("Post Scheduler", "Configuration must have both Facebook App ID and Secret.")
		if err != nil {
			return err
		}

		res.WriteHeader(http.StatusBadRequest)
		res.Write(v)

		return fmt.Errorf("No app ID or Secret sent with request: %v", req.Form)
	}

	// try to get the extended auth token
	if tmpUserToken != "" {
		endpoint := FBAPI + "/oauth/access_token?"
		params := []string{
			"grant_type=fb_exchange_token",
			"client_id=" + appID,
			"client_secret=" + appSecret,
			"fb_exchange_token=" + tmpUserToken,
		}

		uri := endpoint + strings.Join(params, "&")

		client := http.Client{Timeout: time.Second * 5}
		body := &bytes.Buffer{}
		get, err := http.NewRequest(http.MethodGet, uri, body)
		if err != nil {
			v, err := admin.ErrorMessage("Post Scheduler", "Failed to make request object. Please try again.")
			if err != nil {
				return err
			}

			res.WriteHeader(http.StatusInternalServerError)
			res.Write(v)

			return fmt.Errorf("Failed to make GET request with HTTP client for endpoint: %v", uri)
		}

		resp, err := client.Do(get)
		if err != nil {
			v, err := admin.ErrorMessage("Post Scheduler", "Failed to get extended token from Facebook. Make sure your App ID and Secret are correct.")
			if err != nil {
				return err
			}

			res.WriteHeader(http.StatusInternalServerError)
			res.Write(v)

			return fmt.Errorf("Failed to get extended token from Facebook with request data: %v", req.Form)
		}
		defer resp.Body.Close()

		// read response from Facebook
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != http.StatusOK {
			v, err := admin.Error500()
			if err != nil {
				return err
			}

			res.WriteHeader(http.StatusInternalServerError)
			res.Write(v)

			return fmt.Errorf("Failed to get response from Facebook API: %v", req.Form)
		}

		extendedToken := strings.Split(strings.Split(string(b), "=")[1], "&")[0]
		req.Form.Add("facebook_extended_user_token", extendedToken)
	}

	return nil
}

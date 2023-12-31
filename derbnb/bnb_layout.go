package derbnb

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
	"time"

	dreams "github.com/dReam-dApps/dReams"
	"github.com/dReam-dApps/dReams/bundle"
	"github.com/dReam-dApps/dReams/dwidget"
	"github.com/dReam-dApps/dReams/menu"
	"github.com/dReam-dApps/dReams/rpc"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/validation"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	xwidget "fyne.io/x/fyne/widget"
)

type available_dates struct {
	Start int `json:"Start"`
	End   int `json:"End"`
}

var viewing_scid string
var start_date time.Time
var end_date time.Time
var listing_label *widget.Label
var listings_list *widget.List
var booking_list *widget.Tree
var property_list *widget.Tree
var connect_box *dwidget.DeroRpcEntries

// Layout all objects for DerBnb dApp
//   - imported if used as a package
//   - closing if closing signal of main app
//   - w is main window of main app for switch back
//   - background is background content of main app for switch back
func LayoutAllItems(imported bool, d *dreams.AppObject) fyne.CanvasObject {
	var count int
	var image_box *fyne.Container
	var reset_to_main fyne.CanvasObject

	// Delay to catch reset layout
	go func() {
		time.Sleep(time.Second)
		reset_to_main = d.Window.Content()
	}()

	// label for property info
	listing_label = widget.NewLabel("")
	listing_label.Wrapping = fyne.TextWrapWord
	listing_label.Alignment = fyne.TextAlignCenter

	// listed properties for rent
	listings_list = widget.NewList(
		func() int {
			return len(listed_properties)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(listed_properties[i])
		})

	// property images
	var image canvas.Image
	img_forward := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "arrowForward"), func() {
		go func() {
			property_photos.RLock()
			if len(viewing_scid) == 64 && property_photos.data[viewing_scid] != nil {
				if count < len(property_photos.data[viewing_scid])-1 {
					count++
					if url := propertyImageSource(property_photos.data[viewing_scid][count]); url != "" {
						if image, err := dreams.DownloadFile(url, url); err == nil {
							image_box.Objects[0] = &image
							image_box.Refresh()
						} else {
							logger.Errorf("[DerBnb] %s %s\n", url, err)
						}
					}
				} else {
					count = 0
					if url := propertyImageSource(property_photos.data[viewing_scid][count]); url != "" {
						if image, err := dreams.DownloadFile(url, url); err == nil {
							image_box.Objects[0] = &image
							image_box.Refresh()
						} else {
							logger.Errorf("[DerBnb] %s %s\n", url, err)
						}
					}
				}
			} else {
				dialog.NewInformation("No Images", "This property has no images", d.Window).Show()
			}
			property_photos.RUnlock()
		}()
	})

	img_back := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "arrowBack"), func() {
		go func() {
			property_photos.RLock()
			if len(viewing_scid) == 64 && property_photos.data[viewing_scid] != nil {
				if count > 0 {
					count--
					if url := propertyImageSource(property_photos.data[viewing_scid][count]); url != "" {
						if image, err := dreams.DownloadFile(url, url); err == nil {
							image_box.Objects[0] = &image
							image_box.Refresh()
						} else {
							logger.Errorf("[DerBnb] %s %s\n", url, err)
						}
					}
				} else {
					count = len(property_photos.data[viewing_scid]) - 1
					if url := propertyImageSource(property_photos.data[viewing_scid][count]); url != "" {
						if image, err := dreams.DownloadFile(url, url); err == nil {
							image_box.Objects[0] = &image
							image_box.Refresh()
						} else {
							logger.Errorf("[DerBnb] %s %s\n", url, err)
						}
					}
				}
			} else {
				dialog.NewInformation("No Images", "This property has no images", d.Window).Show()
			}
			property_photos.RUnlock()
		}()
	})

	img_forward.Importance = widget.LowImportance
	img_back.Importance = widget.LowImportance

	image_box = container.NewMax(&image)
	image_cont := container.NewBorder(nil, nil, container.NewCenter(img_back), container.NewCenter(img_forward), image_box)

	// request booking arrive and depart dates
	arrive_canvas := canvas.NewText("Arriving:", bundle.TextColor)
	depart_canvas := canvas.NewText("Departing:", bundle.TextColor)

	arrive_reset := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "viewRefresh"), func() {
		start_date = time.Time{}
		arrive_canvas.Text = ("Arriving:")
		arrive_canvas.Refresh()
	})

	depart_reset := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "viewRefresh"), func() {
		end_date = time.Time{}
		depart_canvas.Text = ("Departing:")
		depart_canvas.Refresh()
	})

	arrive_reset.Importance = widget.LowImportance
	depart_reset.Importance = widget.LowImportance

	now := time.Now()

	cal_dates := &trip_date{arriving: arrive_canvas, departing: depart_canvas}
	calendar := xwidget.NewCalendar(now, cal_dates.onSelected)

	arrive_box := container.NewBorder(nil, nil, arrive_reset, nil, arrive_canvas)
	depart_box := container.NewBorder(nil, nil, depart_reset, nil, depart_canvas)

	dates_box := container.NewAdaptiveGrid(2, arrive_box, depart_box)

	listing_label_cont := container.NewScroll(listing_label)
	listing_label_cont.SetMinSize(fyne.NewSize(75, 30))

	layout1_top_split := container.NewHSplit(image_cont, listing_label_cont)

	var viewing_address string
	var request_button, confirm_request_button, cancel_request_button, release_button, cancel_booking_button *widget.Button

	// mint a new property token
	mint_prop := widget.NewButtonWithIcon("Mint", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "contentAdd"), nil)
	mint_prop.Importance = widget.LowImportance

	property_search_entry := widget.NewEntry()
	property_search_entry.SetPlaceHolder("Search properties:")
	property_search_entry.Wrapping = fyne.TextTruncate

	property_search_by := widget.NewRadioGroup([]string{"Country  ", "City"}, nil)
	property_search_by.Horizontal = true
	property_search_by.SetSelected("Country  ")

	property_search_button := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "search"), func() {
		if property_search_entry.Text != "" && rpc.Wallet.IsConnected() {
			viewing_scid = ""
			viewing_address = ""
			listings_list.UnselectAll()
			switch property_search_by.Selected {
			case "Country  ":
				SearchProperties(property_search_entry.Text, false)
			case "City":
				SearchProperties(property_search_entry.Text, true)
			}
		}
	})

	property_clear_button := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "searchReplace"), func() {
		viewing_scid = ""
		viewing_address = ""
		property_search_entry.SetText("")
		searching_properties = false
		listings_list.UnselectAll()
	})

	property_search_bar := container.NewBorder(nil, nil, property_clear_button, nil, property_search_entry)
	property_search_cont := container.NewBorder(nil, nil, property_search_by, property_search_button, property_search_bar)

	mint_prop_spacer := canvas.NewRectangle(color.RGBA{0, 0, 0, 0})
	mint_prop_spacer.SetMinSize(fyne.NewSize(75, 0))

	mint_prop_cont := container.NewHBox(mint_prop, mint_prop_spacer)

	listings_cont := container.NewBorder(container.NewBorder(nil, nil, mint_prop_cont, nil, property_search_cont), nil, nil, nil, listings_list)

	cal_space := canvas.NewRectangle(color.RGBA{0, 0, 0, 0})
	cal_space.SetMinSize(fyne.NewSize(290, 0))
	layout1_bottom_split := container.NewBorder(nil, nil, nil, container.NewMax(cal_space, calendar), listings_cont)

	layout1_split := container.NewVSplit(layout1_top_split, layout1_bottom_split)

	// default ringsize for messages
	ringsize_val := uint64(32)

	// confirmation screen vars
	var confirm_stamp uint64
	var confirm_dates string
	var viewingValidators func()
	var profileValidators func()

	// profile tab layout
	scid_entry := widget.NewEntry()
	scid_entry.SetPlaceHolder("SCID:")
	scid_entry.Validator = validation.NewRegexp(`^\w{64,64}$`, "SCID Not Valid")
	scid_entry.OnChanged = func(s string) {
		profileValidators()
	}

	// price per night entry
	price_entry := dwidget.NewDeroEntry("", 0.1, 5)
	price_entry.SetPlaceHolder("Price:     ")
	price_entry.AllowFloat = true
	price_entry.Validator = validation.NewRegexp(`^\d{1,}\.\d{1,5}$|^[^0]\d{0,}$`, "Float required")
	price_entry.OnChanged = func(s string) {
		profileValidators()
	}

	// damage deposit entry
	deposit_entry := dwidget.NewDeroEntry("", 0.1, 5)
	deposit_entry.AllowFloat = true
	deposit_entry.SetPlaceHolder("Damage deposit:")
	deposit_entry.Validator = validation.NewRegexp(`^\d{1,}\.\d{1,5}$|^[^0]\d{0,}$`, "Float required")
	deposit_entry.OnChanged = func(s string) {
		profileValidators()
	}

	// damage deposit release objects
	release_entry := dwidget.NewDeroEntry("", 0.1, 5)
	release_entry.SetPlaceHolder("Damage amount in Dero:")
	release_entry.Validator = validation.NewRegexp(`^\d{1,}\.\d{1,5}$|^[^0]\d{0,}$`, "Float required")

	comment_entry := widget.NewMultiLineEntry()

	// message objects
	ringsize_select := widget.NewSelect([]string{"16", "32", "64"}, func(s string) {
		switch s {
		case "16":
			ringsize_val = 16
		case "32":
			ringsize_val = 32
		case "64":
			ringsize_val = 64
		default:
			ringsize_val = 32
		}
	})
	ringsize_select.PlaceHolder = "Ringsize:"

	message_cont := container.NewBorder(
		container.NewCenter(ringsize_select),
		nil,
		nil,
		nil,
		comment_entry)

	// user experience rating objects
	owner_slider_label := widget.NewLabel("Owner: 1")
	owner_slider := widget.NewSlider(1, 5)
	owner_slider.Step = 1
	owner_slider.OnChanged = func(f float64) {
		owner_slider_label.SetText(fmt.Sprintf("Owner: %.0f", f))
	}
	rating_border1 := container.NewBorder(owner_slider_label, nil, nil, nil, owner_slider)

	property_slider_label := widget.NewLabel("Property: 1")
	property_slider := widget.NewSlider(1, 5)
	property_slider.Step = 1
	property_slider.OnChanged = func(f float64) {
		property_slider_label.SetText(fmt.Sprintf("Property: %.0f", f))
	}
	rating_border2 := container.NewBorder(property_slider_label, nil, nil, nil, property_slider)

	location_slider_label := widget.NewLabel("Location: 1")
	location_slider := widget.NewSlider(1, 5)
	location_slider.Step = 1
	location_slider.OnChanged = func(f float64) {
		location_slider_label.SetText(fmt.Sprintf("Location: %.0f", f))
	}
	rating_border3 := container.NewBorder(location_slider_label, nil, nil, nil, location_slider)

	overall_slider_label := widget.NewLabel("Overall: 1")
	overall_slider := widget.NewSlider(1, 5)
	overall_slider.Step = 1
	overall_slider.OnChanged = func(f float64) {
		overall_slider_label.SetText(fmt.Sprintf("Overall: %.0f", f))
	}
	rating_border4 := container.NewBorder(overall_slider_label, nil, nil, nil, overall_slider)

	user_rating_cont := container.NewVBox(rating_border1, rating_border2, rating_border3, rating_border4)

	// owner experience rating
	renter_slider_label := widget.NewLabel("Renter: 1")
	renter_slider := widget.NewSlider(1, 5)
	renter_slider.Step = 1
	renter_slider.OnChanged = func(f float64) {
		renter_slider_label.SetText(fmt.Sprintf("Renter: %.0f", f))
	}
	owner_rating_border := container.NewBorder(renter_slider_label, nil, nil, nil, renter_slider)
	owner_rating_cont := container.NewVBox(owner_rating_border)

	var tabs *container.AppTabs
	var confirm_border, confirm_max, max *fyne.Container
	var metadata_label_arr []string
	var available_start_arr, available_end_arr []*widget.Entry
	var new_dates_arr, metadata_entry_arr []*fyne.Container

	// confirmation screen objects
	var confirm_action_int int
	var confirm_action_scid string

	confirm_action_label := widget.NewLabel("")
	confirm_action_label.Wrapping = fyne.TextWrapWord
	confirm_action_label.Alignment = fyne.TextAlignCenter

	derbnb_gif, _ := xwidget.NewAnimatedGifFromResource(ResourceDerbnbGif)
	derbnb_gif.SetMinSize(fyne.NewSize(100, 100))

	// DerBnb SC deposit/withdraw entry and container
	var trvl_border *fyne.Container
	trvl_amt_entry := dwidget.NewDeroEntry("", 10000, 0)
	trvl_amt_entry.SetPlaceHolder("TRVL:")
	trvl_amt_entry.AllowFloat = false
	trvl_amt_entry.Validator = validation.NewRegexp(`^[^0]\d{4,}$`, "Int required")
	trvl_amt_entry.OnChanged = func(s string) {
		if trvl_amt_entry.Validate() == nil {
			if f, err := strconv.ParseInt(s, 10, 64); err == nil {
				if f%10000 != 0 {
					trvl_amt_entry.SetText(fmt.Sprintf("%d", (f/10000)*10000))
				}
			}
		}
	}

	trvl_share_entry := dwidget.NewDeroEntry("", 1, 0)
	trvl_share_entry.SetPlaceHolder("Shares:")
	trvl_share_entry.AllowFloat = false
	trvl_share_entry.Validator = validation.NewRegexp(`^[^0]\d{0,}$`, "Int required")

	var release_check *widget.Check
	var confirm_action, cancel_action *widget.Button
	confirm_action = widget.NewButton("Confirm", func() {
		switch confirm_action_int {
		case 1:
			new_install_scid := UploadBnbTokenContract()
			if new_install_scid == "" {
				confirm_action.Hide()
				cancel_action.Hide()
				confirm_action_label.SetText("Token Not Installed")
				confirm_border.Objects[4] = container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
				d.Window.SetContent(confirm_max)
				time.Sleep(6 * time.Second)
				break
			}

			if file, file_err := os.Create("Bnb-Token " + time.Now().Format(time.UnixDate)); file_err == nil {
				defer file.Close()
				if _, file_err = file.WriteString(new_install_scid); file_err == nil {
					logger.Println("[DerBnb] Token SCID File Saved")
				}
			}

			var set_location *widget.Button
			var balance_confirmed bool

			location_entry_label := widget.NewLabel("Location")
			location_entry_label.Alignment = fyne.TextAlignCenter
			city_entry := widget.NewEntry()
			country_entry := widget.NewEntry()

			city_entry.Validator = validation.NewRegexp(`^\w{2,}`, "String required")
			city_entry.OnChanged = func(s string) {
				if balance_confirmed && city_entry.Validate() == nil && country_entry.Validate() == nil {
					set_location.Show()
				} else {
					set_location.Hide()
				}
			}

			country_entry.Validator = validation.NewRegexp(`^\w{2,}`, "String required")
			country_entry.OnChanged = func(s string) {
				if balance_confirmed && city_entry.Validate() == nil && country_entry.Validate() == nil {
					set_location.Show()
				} else {
					set_location.Hide()
				}
			}

			city_entry.SetPlaceHolder("City:")
			country_entry.SetPlaceHolder("Country:")
			city_entry.Hide()
			country_entry.Hide()

			var location_is_set bool
			set_label := widget.NewLabel("")
			set_location = widget.NewButton("Set Location", func() {
				data := location_data{}
				data.City = city_entry.Text
				data.Country = country_entry.Text
				if mar, err := json.Marshal(data); err == nil {
					set_location.Hide()
					set_label.SetText("Wait for block")
					set_label.Show()
					StoreLocation(new_install_scid, string(mar))
					go func() {
						i := 0
						time.Sleep(5 * time.Second)
						for set_location.Hidden && !menu.ClosingApps() {
							city, country := getLocation(new_install_scid)
							if city != "" && country != "" {
								location_is_set = true
								set_label.SetText("Location is now set")
								return
							}

							i++
							if i > 35 {
								set_label.SetText("Location not set, try again")
								set_location.Show()
								return
							}
							time.Sleep(2 * time.Second)
						}
					}()
				}
			})
			set_location.Hide()

			back_button := widget.NewButtonWithIcon("Back", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "cancel"), func() {
				confirm_action_int = 0
				confirm_action_scid = ""
				comment_entry.SetPlaceHolder("")
				comment_entry.SetText("")
				release_entry.SetText("")
				release_check.SetChecked(false)
				confirm_action.Show()
				cancel_action.Show()
				derbnb_gif.Stop()
				menu.RestartGif(menu.Gnomes.Icon_ind)
				d.Window.SetContent(reset_to_main)
			})

			location_entry_cont := container.NewVBox(container.NewAdaptiveGrid(2, city_entry, country_entry), container.NewCenter(set_label), set_location)

			done_button := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "confirm"), func() {
				confirm_border.Objects[4] = container.NewVScroll(container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer(), placeMetadataObjects(metadata_label_arr, metadata_entry_arr)))
				confirm_action_label.SetText(fmt.Sprintf("Set property info\n\nSCID: %s\n\n", scid_entry.Text))
				confirm_action_int = 14
				confirm_border.Refresh()
				d.Window.SetContent(confirm_max)
			})
			done_button.Hide()

			copy_button := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "contentCopy"), func() {
				d.Window.Clipboard().SetContent(new_install_scid)
				if location_is_set {
					scid_entry.SetText(new_install_scid)
					done_button.Show()
				}
			})

			install_box := container.NewAdaptiveGrid(2, container.NewMax(done_button), container.NewBorder(nil, nil, nil, back_button, copy_button))

			confirm_alpha := canvas.NewRectangle(color.RGBA{0, 0, 0, 150})
			if bundle.AppColor == color.White {
				confirm_alpha = canvas.NewRectangle(color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xaa})
			}

			location_max := container.NewMax(d.Background, confirm_alpha)
			if imported {
				confirm_alpha2 := canvas.NewRectangle(color.RGBA{0, 0, 0, 120})
				if bundle.AppColor == color.White {
					confirm_alpha2 = canvas.NewRectangle(color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x55})
				}
				location_max.Add(confirm_alpha2)
			}

			if len(new_install_scid) != 64 {
				confirm_action_label.SetText(fmt.Sprintf("Token Install Failed \n\n%s", new_install_scid))
				d.Window.SetContent(container.NewMax(location_max, container.NewBorder(derbnb_gif, install_box, nil, nil, confirm_action_label)))
			} else {
				confirm_action_label.SetText(fmt.Sprintf("Waiting for Token Balance\n\nSCID: %s\n\nBalance: %d", new_install_scid, 0))
				d.Window.SetContent(container.NewMax(location_max, container.NewBorder(derbnb_gif, install_box, nil, nil, container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer(), location_entry_cont, layout.NewSpacer()))))
				go func() {
					for !balance_confirmed {
						if menu.ClosingApps() {
							return
						}
						if bal := rpc.TokenBalance(new_install_scid); bal > 0 {
							balance_confirmed = true
						}
					}
					confirm_action_label.SetText(fmt.Sprintf("Token Installed\n\nSCID: %s\n\nBalance: %d\n\n\nContinue to Set Location", new_install_scid, 1))
					city_entry.Show()
					country_entry.Show()
				}()
			}
			return

		case 2:
			ListProperty(scid_entry.Text, rpc.ToAtomic(price_entry.Text, 5), rpc.ToAtomic(deposit_entry.Text, 5), true)
		case 3:
			RemoveProperty(scid_entry.Text)
		case 4:
			ConfirmBooking(scid_entry.Text, confirm_stamp)
			confirm_request_button.Hide()
		case 5:
			ReleaseDamageDeposit(scid_entry.Text, comment_entry.Text, confirm_stamp, rpc.ToAtomic(release_entry.Text, 5))
			release_button.Hide()
		case 6:
			ListProperty(scid_entry.Text, rpc.ToAtomic(price_entry.Text, 5), rpc.ToAtomic(deposit_entry.Text, 5), false)
		case 7:
			// case removed for v0.9.6
		case 8:
			CancelBooking(confirm_action_scid, confirm_stamp)
			cancel_request_button.Hide()
			cancel_booking_button.Hide()
		case 9:
			RateExperience(confirm_action_scid, confirm_stamp, 0, uint64(owner_slider.Value), uint64(property_slider.Value), uint64(location_slider.Value), uint64(overall_slider.Value))
		case 10:
			RateExperience(confirm_action_scid, confirm_stamp, uint64(renter_slider.Value), 0, 0, 0, 0)
		case 11:
			rpc.SendMessage(viewing_address, comment_entry.Text, ringsize_val)
		case 12:
			var new_dates []available_dates
			for i, cont := range new_dates_arr {
				if !cont.Hidden {
					trim_start := strings.TrimPrefix(available_start_arr[i].Text, "Starting: ")
					add_these := available_dates{}
					if date, err := time.Parse(TIME_FORMAT, trim_start); err == nil {
						add_these.Start = int(date.Unix())
						trim_end := strings.TrimPrefix(available_end_arr[i].Text, "Ending: ")
						if date, err := time.Parse(TIME_FORMAT, trim_end); err == nil {
							add_these.End = int(date.Unix())
						}
					}

					if add_these.Start > 0 && add_these.End > 0 {
						new_dates = append(new_dates, add_these)
					}
				}
			}

			if mar, err := json.Marshal(new_dates); err == nil {
				ChangeAvailability(scid_entry.Text, string(mar))
			}

		case 13:
			start := uint64(start_date.Unix())
			end := uint64(end_date.Unix())
			amt := current_price*((end-start)/84600) + current_deposit
			RequestBooking(viewing_scid, uint64(time.Now().Unix()), start, end, amt)
		case 14:
			metadata := property_data{}
			for i, cont := range metadata_entry_arr {
				switch i {
				case 0:
					metadata.Surface = rpc.StringToInt(cont.Objects[0].(*dwidget.DeroAmts).Text)
				case 1:
					metadata.Style = cont.Objects[0].(*widget.Select).Selected
				case 2:
					metadata.NumberOfBedrooms = rpc.StringToInt(cont.Objects[0].(*dwidget.DeroAmts).Text)
				case 3:
					metadata.MaxNumberOfGuests = rpc.StringToInt(cont.Objects[0].(*dwidget.DeroAmts).Text)
				case 4:
					for _, w := range cont.Objects {
						if w.(*widget.Entry).Text != "" {
							metadata.Photos = append(metadata.Photos, w.(*widget.Entry).Text)
						}
					}
				case 5:
					metadata.Description = cont.Objects[0].(*widget.Entry).Text
				case 6:
					metadata.CleaningFee = rpc.StringToInt(cont.Objects[0].(*dwidget.DeroAmts).Text)
				case 7:
					metadata.MinimumStay = rpc.StringToInt(cont.Objects[0].(*dwidget.DeroAmts).Text)
				case 8:
					metadata.MaximumStay = rpc.StringToInt(cont.Objects[0].(*dwidget.DeroAmts).Text)
				case 9:
					metadata.Name = cont.Objects[0].(*widget.Entry).Text
				case 10:
					metadata.Share = cont.Objects[0].(*widget.Select).Selected
				case 11:
					metadata.Rules = cont.Objects[0].(*widget.Entry).Text
				case 12:
					metadata.NumberOfBathrooms = rpc.StringToInt(cont.Objects[0].(*dwidget.DeroAmts).Text)
				case 13:
					metadata.Pets = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 14:
					metadata.Amenities.Wifi = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 15:
					metadata.Amenities.TV = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 16:
					metadata.Amenities.Kitchen = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 17:
					metadata.Amenities.Washer = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 18:
					metadata.Amenities.Parking = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 19:
					metadata.Amenities.AirConditioner = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 20:
					metadata.Amenities.Workspace = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 21:
					metadata.Amenities.Pool = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 22:
					metadata.Amenities.HotTub = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 23:
					metadata.Amenities.BBQ = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 24:
					metadata.Amenities.OutdoorDining = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 25:
					metadata.Amenities.FirePit = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 26:
					metadata.Amenities.GamesRoom = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 27:
					metadata.Amenities.ExerciseEquip = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 28:
					metadata.Amenities.LakeAccess = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 29:
					metadata.Amenities.BeachAccess = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 30:
					metadata.Amenities.SmokeAlarm = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				case 31:
					metadata.Amenities.FireExtinguisher = amenityValue(cont.Objects[0].(*widget.RadioGroup).Selected)
				default:

				}
			}

			if mar, err := json.Marshal(metadata); err == nil {
				UpdateMetadata(scid_entry.Text, string(mar))
			}
		default:

		}
		confirm_action_int = 0
		confirm_action_scid = ""
		comment_entry.SetPlaceHolder("")
		comment_entry.SetText("")
		release_entry.SetText("")
		release_check.SetChecked(false)
		confirm_action.Show()
		cancel_action.Show()
		derbnb_gif.Stop()
		menu.RestartGif(menu.Gnomes.Icon_ind)
		d.Window.SetContent(reset_to_main)
	})

	cancel_action = widget.NewButton("Cancel", func() {
		confirm_action_int = 0
		comment_entry.SetPlaceHolder("")
		comment_entry.SetText("")
		release_entry.SetText("")
		release_check.SetChecked(false)
		confirm_action.Show()
		derbnb_gif.Stop()
		menu.RestartGif(menu.Gnomes.Icon_ind)
		d.Window.SetContent(reset_to_main)
	})

	confirm_action_cont := container.NewAdaptiveGrid(2, container.NewMax(confirm_action), cancel_action)
	confirm_border = container.NewBorder(
		derbnb_gif,
		confirm_action_cont,
		layout.NewSpacer(),
		layout.NewSpacer(),
		layout.NewSpacer(),
	)

	confirm_alpha := canvas.NewRectangle(color.RGBA{0, 0, 0, 150})
	if bundle.AppColor == color.White {
		confirm_alpha = canvas.NewRectangle(color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xaa})
	}

	confirm_max = container.NewMax(d.Background, confirm_alpha)
	if imported {
		confirm_alpha2 := canvas.NewRectangle(color.RGBA{0, 0, 0, 120})
		if bundle.AppColor == color.White {
			confirm_alpha2 = canvas.NewRectangle(color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x55})
		}
		confirm_max.Add(confirm_alpha2)
	}

	confirm_max.Add(confirm_border)

	mint_prop.OnTapped = func() {
		derbnb_gif.Start()
		confirm_border.Objects[4] = container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
		minting_fee := float64(rpc.ListingFee) / 100000
		gas_fee := float64(0.015)
		confirm_action_label.SetText(fmt.Sprintf("Mint a new property SCID\n\nMinting fee is %.5f Dero\n\nTotal transaction will be %.5f Dero (%.5f gas fee for contract install)\n\nAfter minting you will be prompted to set your property location, do not close the app until this step is completed", minting_fee, minting_fee+gas_fee, gas_fee))
		confirm_action_int = 1
		d.Window.SetContent(confirm_max)
	}

	release_check = widget.NewCheck("", func(b bool) {
		f, err := strconv.ParseFloat(release_entry.Text, 64)
		if err != nil {
			return
		}

		if b && f > 0 && len(comment_entry.Text) < 3 {
			dialog.NewInformation("Needs Comment", "Add a comment for damage description", d.Window).Show()
			release_check.SetChecked(false)
			return
		}

		if b {
			confirm_action.Show()
			comment_entry.SetPlaceHolder("")
			comment_entry.Disable()
			release_entry.Disable()
		} else {
			confirm_action.Hide()
			comment_entry.SetPlaceHolder("Damage Comments:")
			comment_entry.Enable()
			release_entry.Enable()
		}
	})
	release_check.Disable()

	release_entry.OnChanged = func(s string) {
		if release_entry.Validate() != nil {
			release_check.Disable()
		} else {
			release_check.Enable()
		}
	}

	release_cont := container.NewBorder(
		nil,
		container.NewBorder(nil, nil, nil, release_check, release_entry),
		nil,
		nil,
		comment_entry)

	// bnb contract controls
	request_button = widget.NewButton("Request Booking", func() {
		if viewing_scid != "" && current_price != 0 && current_deposit != 0 && !start_date.IsZero() && !end_date.IsZero() && rpc.Wallet.IsConnected() {
			derbnb_gif.Start()
			start := uint64(start_date.Unix())
			end := uint64(end_date.Unix())
			amt := current_price*((end-start)/84600) + current_deposit
			confirm_border.Objects[4] = container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
			price_str := fmt.Sprintf("%.5f", float64(current_price)/100000)
			dep_str := fmt.Sprintf("%.5f", float64(current_deposit)/100000)
			amt_str := fmt.Sprintf("%.5f", float64(amt)/100000)
			location := makeLocationString(viewing_scid)
			confirm_action_label.SetText(fmt.Sprintf("Request booking\n\nSCID: %s\n\nLocation: %s\n\nDaily rate of: %s Dero\n\nDamage deposit: %s Dero\n\nArriving: %s\n\nDeparting: %s\n\nTotal: %s Dero", viewing_scid, location, price_str, dep_str, start_date.Format(TIME_FORMAT), end_date.Format(TIME_FORMAT), amt_str))
			confirm_action_int = 13
			d.Window.SetContent(confirm_max)
		}
	})

	// DerBnb SC deposit/withdraw controls
	trvl_dep := widget.NewButton("Deposit TRVL", func() {
		if trvl_amt_entry.Validate() == nil {
			trvl_amt, err := strconv.ParseInt(trvl_amt_entry.Text, 10, 64)
			if err != nil || trvl_amt%10000 != 0 {
				dialog.NewInformation("Error", "TRVL amount error", d.Window).Show()
				return
			}

			if rpc.Wallet.ReadTokenBalance("TRVL") < uint64(trvl_amt) {
				dialog.NewInformation("Not Enough TRVL", "You do not have that much TRVL", d.Window).Show()
				return
			}

			info := fmt.Sprintf("Deposit %s TRVL\n\nYou will get %d Shares", trvl_amt_entry.Text, trvl_amt/10000)
			dialog.NewConfirm("Deposit TRVL", info, func(b bool) {
				if b {
					DepositToDerBnb(true, uint64(trvl_amt))
				}
			}, d.Window).Show()
		}
	})

	trvl_sell := widget.NewButton("Sell Shares", func() {
		if trvl_share_entry.Validate() == nil {
			check, err := strconv.ParseInt(trvl_share_entry.Text, 10, 64)
			if err != nil {
				dialog.NewInformation("Error", "Share amount error", d.Window).Show()
				return
			}

			if shares, _, _ := getUserShares(); shares < uint64(check) {
				dialog.NewInformation("Not Enough Shares", "You do not have that many shares", d.Window).Show()
				return
			}

			info := fmt.Sprintf("Selling %s Shares\n\nYou will get %s0000 TRVL Tokens", trvl_share_entry.Text, trvl_share_entry.Text)
			dialog.NewConfirm("Sell Shares", info, func(b bool) {
				if b {
					SellDerBnbShares(uint64(check))
				}
			}, d.Window).Show()
		}
	})

	trvl_withdraw := widget.NewButton("Withdraw", func() {
		shares, epoch, treasury := getUserShares()
		if shares == 0 {
			dialog.NewInformation("No Shares", "You do not have any shares", d.Window).Show()
			return
		}

		next_withdraw := uint64(time.Now().Unix()-1684428835) / 2629743
		if next_withdraw > epoch {
			pay := (shares * treasury) / 1000000
			info := fmt.Sprintf("You have %d Shares\n\nYou will get %0.5f Dero", shares, float64(pay)/100000)
			dialog.NewConfirm("Withdraw Dero", info, func(b bool) {
				if b {
					WithdrawFromDerBnb()
				}
			}, d.Window).Show()

			return
		}

		dialog.NewInformation("Not Ready", "Your next withdraw is not ready", d.Window).Show()
	})

	var trvl_exit bool
	trvl_back := widget.NewButton("Back", func() {
		trvl_exit = true
		derbnb_gif.Stop()
		menu.RestartGif(menu.Gnomes.Icon_ind)
		d.Window.SetContent(reset_to_main)
	})

	trvl_border = container.NewVBox(
		layout.NewSpacer(),
		container.NewAdaptiveGrid(4, trvl_amt_entry, trvl_share_entry, layout.NewSpacer(), layout.NewSpacer()),
		container.NewAdaptiveGrid(4, trvl_dep, trvl_sell, trvl_withdraw, trvl_back))

	trvl_button := widget.NewButton("TRVL Tokens", func() {
		derbnb_gif.Start()
		trvl_amt_entry.SetText("")
		trvl_share_entry.SetText("")
		confirm_border.Objects[4] = container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
		confirm_border.Objects[2] = trvl_border
		d.Window.SetContent(confirm_max)
		go func() {
			for !menu.ClosingApps() && !trvl_exit {
				if !rpc.Wallet.IsConnected() {
					derbnb_gif.Stop()
					menu.RestartGif(menu.Gnomes.Icon_ind)
					d.Window.SetContent(reset_to_main)
					break
				}
				shares, epoch, _ := getUserShares()
				next_withdraw := uint64(time.Now().Unix()-1684428835) / 2629743
				if shares == 0 {
					epoch = next_withdraw
				}
				confirm_action_label.SetText(fmt.Sprintf("Shares allow you to withdraw Dero from the DerBnb treasury each month\n\n10000 TRVL = 1 Share\n\nTRVL Balance: %d\n\nShares: %d\n\nWithdraw Available: %t", rpc.Wallet.ReadTokenBalance("TRVL"), shares, next_withdraw > epoch))
				time.Sleep(time.Second)
			}
			trvl_exit = false
		}()
	})

	message_listing := widget.NewButton("Message", func() {
		if len(viewing_address) == 66 && viewing_address[0:4] == "dero" {
			derbnb_gif.Start()
			comment_entry.SetPlaceHolder("Message:")
			cont := container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
			confirm_border.Objects[4] = container.NewVSplit(cont, message_cont)
			confirm_action_label.SetText(fmt.Sprintf("Sending message to:\n\n%s", viewing_address))
			confirm_action_int = 11
			confirm_border.Refresh()
			d.Window.SetContent(confirm_max)
		}
	})
	message_listing.Hide()

	listings_list.OnSelected = func(id widget.ListItemID) {
		go func() {
			image_box.Objects[0] = canvas.NewImageFromImage(nil)
			image_box.Refresh()
			split := strings.Split(listed_properties[id], "   ")
			if len(split) > 1 {
				scid := split[1]
				viewing_scid = scid
				count = 0
				getImages(scid)
				request_button.Show()
				message_listing.Show()
				if len(viewing_scid) != 64 {
					request_button.Hide()
					message_listing.Hide()
				}
				viewing_address = getOwnerAddress(split[1])
				property_photos.RLock()
				listing_label.SetText(getInfo(scid))
				if property_photos.data[scid] != nil {
					if url := propertyImageSource(property_photos.data[scid][0]); url != "" {
						if image, err := dreams.DownloadFile(url, url); err == nil {
							image_box.Objects[0] = &image
							image_box.Refresh()
						} else {
							logger.Errorf("[DerBnb] %s %s\n", url, err)
							image_box.Objects[0] = canvas.NewImageFromImage(nil)
							image_box.Refresh()
						}
					}
				} else {
					image_box.Objects[0] = canvas.NewImageFromImage(nil)
					image_box.Refresh()
				}
				property_photos.RUnlock()
			}
		}()
	}

	list_button := widget.NewButtonWithIcon("List", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "contentAdd"), func() {
		if scid_entry.Validate() == nil && price_entry.Validate() == nil && deposit_entry.Validate() == nil {
			if location := makeLocationString(scid_entry.Text); location != "" {
				if data := getMetadata(scid_entry.Text); data != nil {
					derbnb_gif.Start()
					data_str1 := fmt.Sprintf("Sq Meters: %d\n\nStyle: %s\n\nBedrooms: %d\n\nMax guests: %d\n\nDescription: %s\n\nCleaning fee: %d\n\n", data.Surface, data.Style, data.NumberOfBedrooms, data.MaxNumberOfGuests, data.Description, data.CleaningFee)
					data_str2 := fmt.Sprintf("Minimum stay: %d\n\nMaximum stay: %d\n\nName: %s\n\nShare: %s\n\nRules: %s\n\nBathrooms: %d\n\n", data.MinimumStay, data.MaximumStay, data.Name, data.Share, data.Rules, data.NumberOfBathrooms)
					amen_str1 := fmt.Sprintf("Pets: %s\n\nWifi: %s\n\nTV: %s\n\nKitchen: %s\n\nWasher: %s\n\nFree parking: %s\n\nAir conditioner: %s\n\n", amenityDisplay(data.Pets), amenityDisplay(data.Amenities.Wifi), amenityDisplay(data.Amenities.TV), amenityDisplay(data.Amenities.Kitchen), amenityDisplay(data.Amenities.Washer), amenityDisplay(data.Amenities.Parking), amenityDisplay(data.Amenities.AirConditioner))
					amen_str2 := fmt.Sprintf("Workspace: %s\n\nPool access: %s\n\nHot tub: %s\n\nBBQ: %s\n\nOutdoor dining: %s\n\nFire pit: %s\n\nGames room: %s\n\n", amenityDisplay(data.Amenities.Workspace), amenityDisplay(data.Amenities.Pool), amenityDisplay(data.Amenities.HotTub), amenityDisplay(data.Amenities.BBQ), amenityDisplay(data.Amenities.OutdoorDining), amenityDisplay(data.Amenities.FirePit), amenityDisplay(data.Amenities.GamesRoom))
					amen_str3 := fmt.Sprintf("Exercise equipment: %s\n\nLake access: %s\n\nBeach access: %s\n\nSmoke alarm: %s\n\nFire extinguisher: %s", amenityDisplay(data.Amenities.ExerciseEquip), amenityDisplay(data.Amenities.LakeAccess), amenityDisplay(data.Amenities.BeachAccess), amenityDisplay(data.Amenities.SmokeAlarm), amenityDisplay(data.Amenities.FireExtinguisher))
					confirm_border.Objects[4] = container.NewVScroll(container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer()))
					price_str := price_entry.Text
					if price_fl, err := strconv.ParseFloat(price_str, 64); err == nil {
						price_str = fmt.Sprintf("%.5f", float64(price_fl))
					}

					dep_str := deposit_entry.Text
					if dep_fl, err := strconv.ParseFloat(dep_str, 64); err == nil {
						dep_str = fmt.Sprintf("%.5f", float64(dep_fl))
					}

					confirm_action_label.SetText(fmt.Sprintf("Listing property\n\nSCID: %s\n\n%s\n\nDaily rate of: %s Dero\n\nDamage deposit: %s Dero\n\n%s%s%s%s%s", scid_entry.Text, location, price_str, dep_str, data_str1, data_str2, amen_str1, amen_str2, amen_str3))
					confirm_action_int = 2
					d.Window.SetContent(confirm_max)
				} else {
					logger.Warnln("[DerBnb] Your property needs metadata")
					info_message := dialog.NewInformation("Add Property Info", "Your property information needs to be added before it can be listed", d.Window)
					info_message.SetDismissText("Add Info")
					info_message.SetOnClosed(func() {
						derbnb_gif.Start()
						confirm_border.Objects[4] = container.NewVScroll(container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer(), placeMetadataObjects(metadata_label_arr, metadata_entry_arr)))
						confirm_action_label.SetText(fmt.Sprintf("Set property info\n\nSCID: %s", scid_entry.Text))
						confirm_action_int = 14
						confirm_border.Refresh()
						d.Window.SetContent(confirm_max)
					})
					info_message.Show()
				}
			} else {
				logger.Warnln("[DerBnb] Your property needs a location")
				dialog.NewInformation("Add Property Location", "Your property needs a location added before it can be listed", d.Window).Show()
			}
		}
	})

	var set_location_button *widget.Button
	set_location_button = widget.NewButtonWithIcon("Location", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "contentRedo"), func() {
		if scid_entry.Validate() == nil && rpc.TokenBalance(scid_entry.Text) == 1 {
			var set_location *widget.Button

			location_entry_label := widget.NewLabel("Location")
			location_entry_label.Alignment = fyne.TextAlignCenter
			city_entry := widget.NewEntry()
			country_entry := widget.NewEntry()

			city_entry.Validator = validation.NewRegexp(`^\w{2,}`, "String required")
			city_entry.OnChanged = func(s string) {
				if city_entry.Validate() == nil && country_entry.Validate() == nil {
					set_location.Show()
				} else {
					set_location.Hide()
				}
			}

			country_entry.Validator = validation.NewRegexp(`^\w{2,}`, "String required")
			country_entry.OnChanged = func(s string) {
				if city_entry.Validate() == nil && country_entry.Validate() == nil {
					set_location.Show()
				} else {
					set_location.Hide()
				}
			}

			city_entry.SetPlaceHolder("City:")
			country_entry.SetPlaceHolder("Country:")

			var location_is_set bool
			set_label := widget.NewLabel("")
			set_location = widget.NewButton("Set Location", func() {
				data := location_data{}
				data.City = city_entry.Text
				data.Country = country_entry.Text
				if mar, err := json.Marshal(data); err == nil {
					set_location.Hide()
					set_label.SetText("Wait for block")
					set_label.Show()
					StoreLocation(scid_entry.Text, string(mar))
					go func() {
						i := 0
						time.Sleep(5 * time.Second)
						for set_location.Hidden {
							city, country := getLocation(scid_entry.Text)
							if city != "" && country != "" {
								location_is_set = true
								set_label.SetText("Location is now set")
								return
							}

							i++
							if i > 28 {
								set_label.SetText("Location not set, try again")
								set_location.Show()
								return
							}
							time.Sleep(2 * time.Second)
						}
					}()
				}
			})
			set_location.Hide()

			location_entry_cont := container.NewVBox(container.NewAdaptiveGrid(2, city_entry, country_entry), container.NewCenter(set_label), set_location)

			cancel_location_button := widget.NewButton("Cancel", func() {
				confirm_action_int = 0
				comment_entry.SetPlaceHolder("")
				comment_entry.SetText("")
				release_entry.SetText("")
				release_check.SetChecked(false)
				confirm_action.Show()
				set_location_button.Hide()
				d.Window.SetContent(reset_to_main)
			})

			copy_location_button := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "contentCopy"), func() {
				d.Window.Clipboard().SetContent(scid_entry.Text)
				if location_is_set {
					scid_entry.SetText(scid_entry.Text)
				}
			})

			install_box := container.NewAdaptiveGrid(2, copy_location_button, cancel_location_button)
			confirm_action_label.SetText(fmt.Sprintf("Set Location for SCID\n\n%s", scid_entry.Text))
			location_max := container.NewMax(d.Background, confirm_alpha)
			if imported {
				confirm_alpha2 := canvas.NewRectangle(color.RGBA{0, 0, 0, 120})
				if bundle.AppColor == color.White {
					confirm_alpha2 = canvas.NewRectangle(color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x55})
				}
				location_max.Add(confirm_alpha2)
			}

			d.Window.SetContent(container.NewMax(location_max, container.NewBorder(nil, install_box, nil, nil, container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer(), location_entry_cont, layout.NewSpacer()))))
		}
	})

	remove_button := widget.NewButtonWithIcon("Remove", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "contentRemove"), func() {
		if scid_entry.Validate() == nil {
			derbnb_gif.Start()
			confirm_border.Objects[4] = container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
			confirm_action_label.SetText(fmt.Sprintf("Remove property\n\nSCID: %s", scid_entry.Text))
			confirm_action_int = 3
			d.Window.SetContent(confirm_max)
		}
	})

	confirm_request_button = widget.NewButton("Confirm Request", func() {
		if scid_entry.Validate() == nil {
			if confirm_stamp != 0 && viewing_address != "" {
				derbnb_gif.Start()
				confirm_border.Objects[4] = container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
				confirm_action_label.SetText(fmt.Sprintf("Confirm booking Request\n\nBooking ID: %d\n\nSCID: %s\n\nRenter: %s\n\n%s", confirm_stamp, scid_entry.Text, viewing_address, confirm_dates))
				confirm_action_int = 4
				confirm_request_button.Hide()
				cancel_request_button.Hide()
				d.Window.SetContent(confirm_max)
			}
		}
	})

	cancel_request_button = widget.NewButton("Reject Request", func() {
		if scid_entry.Validate() == nil {
			if confirm_stamp != 0 && viewing_address != "" {
				derbnb_gif.Start()
				confirm_border.Objects[4] = container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
				confirm_action_scid = scid_entry.Text
				confirm_action_label.SetText(fmt.Sprintf("Reject booking request\n\nBooking ID: %d\n\nSCID: %s\n\nRenter: %s", confirm_stamp, confirm_action_scid, viewing_address))
				confirm_action_int = 8
				d.Window.SetContent(confirm_max)
			}
		}
	})

	release_button = widget.NewButton("Release Deposit", func() {
		if scid_entry.Validate() == nil {
			if confirm_stamp != 0 {
				derbnb_gif.Start()
				comment_entry.SetPlaceHolder("Damage Comments:")
				confirm_action.Hide()
				confirm_border.Objects[4] = container.NewVSplit(container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer()), release_cont)
				confirm_action_label.SetText(fmt.Sprintf("Release damage deposit\n\nBooking ID: %d\n\nSCID: %s", confirm_stamp, scid_entry.Text))
				confirm_action_int = 5
				d.Window.SetContent(confirm_max)
			}
		}
	})

	change_price_button := widget.NewButton("Update Prices", func() {
		if scid_entry.Validate() == nil && price_entry.Validate() == nil && deposit_entry.Validate() == nil {
			derbnb_gif.Start()
			new_price, err := strconv.ParseFloat(price_entry.Text, 64)
			if err != nil {
				dialog.NewInformation("Error", "Price amount error", d.Window).Show()
				return
			}

			new_dep, err := strconv.ParseFloat(deposit_entry.Text, 64)
			if err != nil {
				dialog.NewInformation("Error", "Deposit amount error", d.Window).Show()
				return
			}

			confirm_border.Objects[4] = container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
			confirm_action_label.SetText(fmt.Sprintf("Update prices\n\nSCID: %s\n\nNew daily price: %.5f Dero\n\nNew damage deposit: %.5f Dero", scid_entry.Text, new_price, new_dep))
			confirm_action_int = 6
			d.Window.SetContent(confirm_max)
		}
	})

	cancel_booking_button = widget.NewButton("Cancel Booking", func() {
		if len(confirm_action_scid) == 64 && confirm_stamp != 0 {
			derbnb_gif.Start()
			confirm_border.Objects[4] = container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
			confirm_action_label.SetText(fmt.Sprintf("Cancel booking\n\nBooking ID: %d\n\nSCID: %s", confirm_stamp, confirm_action_scid))
			confirm_action_int = 8
			d.Window.SetContent(confirm_max)
		}
	})

	rate_booking_button := widget.NewButton("Rate Booking", func() {
		if len(confirm_action_scid) == 64 && confirm_stamp != 0 {
			derbnb_gif.Start()
			cont := container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
			confirm_border.Objects[4] = container.NewVSplit(cont, user_rating_cont)
			confirm_action_label.SetText(fmt.Sprintf("Rate booking\n\nBooking ID: %d\n\nSCID: %s", confirm_stamp, confirm_action_scid))
			confirm_action_int = 9
			confirm_border.Refresh()
			d.Window.SetContent(confirm_max)
		}
	})

	rate_renter_button := widget.NewButton("Rate Renter", func() {
		if scid_entry.Validate() == nil {
			if confirm_stamp != 0 && viewing_address != "" {
				derbnb_gif.Start()
				cont := container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
				confirm_border.Objects[4] = container.NewVSplit(cont, owner_rating_cont)
				confirm_action_scid = scid_entry.Text
				confirm_action_label.SetText(fmt.Sprintf("Rate renter\n\nBooking ID: %d\n\nSCID: %s\n\nRenter: %s", confirm_stamp, confirm_action_scid, viewing_address))
				confirm_action_int = 10
				confirm_border.Refresh()
				d.Window.SetContent(confirm_max)
			}
		}
	})

	send_message := widget.NewButton("Message", func() {
		if len(viewing_address) == 66 && viewing_address[0:4] == "dero" {
			derbnb_gif.Start()
			comment_entry.SetPlaceHolder("Message:")
			cont := container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer())
			confirm_border.Objects[4] = container.NewVSplit(cont, message_cont)
			confirm_action_label.SetText(fmt.Sprintf("Sending message to:\n\n%s", viewing_address))
			confirm_action_int = 11
			confirm_border.Refresh()
			d.Window.SetContent(confirm_max)
		}
	})

	// set availability objects
	available_start_validation := validation.NewTime("Starting: " + TIME_FORMAT)
	available_start_entry := widget.NewEntry()
	available_start_entry1 := widget.NewEntry()
	available_start_entry2 := widget.NewEntry()
	available_start_entry3 := widget.NewEntry()
	available_start_entry.Disable()
	available_start_entry1.Disable()
	available_start_entry2.Disable()
	available_start_entry3.Disable()
	available_start_entry1.Hide()
	available_start_entry2.Hide()
	available_start_entry3.Hide()
	available_start_entry.Validator = available_start_validation
	available_start_entry1.Validator = available_start_validation
	available_start_entry2.Validator = available_start_validation
	available_start_entry3.Validator = available_start_validation
	available_start_arr = []*widget.Entry{available_start_entry, available_start_entry1, available_start_entry2, available_start_entry3}

	available_end_validation := validation.NewTime("Ending: " + TIME_FORMAT)
	available_end_entry := widget.NewEntry()
	available_end_entry1 := widget.NewEntry()
	available_end_entry2 := widget.NewEntry()
	available_end_entry3 := widget.NewEntry()
	available_end_entry.Disable()
	available_end_entry1.Disable()
	available_end_entry2.Disable()
	available_end_entry3.Disable()
	available_end_entry1.Hide()
	available_end_entry2.Hide()
	available_end_entry3.Hide()
	available_end_entry.Validator = available_end_validation
	available_end_entry1.Validator = available_end_validation
	available_end_entry2.Validator = available_end_validation
	available_end_entry3.Validator = available_end_validation
	available_end_arr = []*widget.Entry{available_end_entry, available_end_entry1, available_end_entry2, available_end_entry3}

	available_start_reset := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "viewRefresh"), func() {
		available_start_entry.SetText("")
	})

	available_end_reset := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "viewRefresh"), func() {
		available_end_entry.SetText("")
	})

	available_start_reset1 := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "viewRefresh"), func() {
		available_start_entry1.SetText("")
	})

	available_end_reset1 := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "viewRefresh"), func() {
		available_end_entry1.SetText("")
	})

	available_start_reset2 := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "viewRefresh"), func() {
		available_start_entry2.SetText("")
	})

	available_end_reset2 := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "viewRefresh"), func() {
		available_end_entry2.SetText("")
	})

	available_start_reset3 := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "viewRefresh"), func() {
		available_start_entry3.SetText("")
	})

	available_end_reset3 := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "viewRefresh"), func() {
		available_end_entry3.SetText("")
	})

	available_start_reset.Importance = widget.LowImportance
	available_start_reset1.Importance = widget.LowImportance
	available_start_reset2.Importance = widget.LowImportance
	available_start_reset3.Importance = widget.LowImportance

	available_end_reset.Importance = widget.LowImportance
	available_end_reset1.Importance = widget.LowImportance
	available_end_reset2.Importance = widget.LowImportance
	available_end_reset3.Importance = widget.LowImportance

	available_d := &add_dates{starting: available_start_arr, ending: available_end_arr}
	available_c := xwidget.NewCalendar(now, available_d.onSelected)

	available_start_box := container.NewBorder(nil, nil, nil, available_start_reset, available_start_arr[0])
	available_end_box := container.NewBorder(nil, nil, layout.NewSpacer(), available_end_reset, available_end_arr[0])
	available_dates_box := container.NewAdaptiveGrid(2, available_start_box, available_end_box)

	available_start_box1 := container.NewBorder(nil, nil, nil, available_start_reset1, available_start_arr[1])
	available_end_box1 := container.NewBorder(nil, nil, layout.NewSpacer(), available_end_reset1, available_end_arr[1])
	available_dates_box1 := container.NewAdaptiveGrid(2, available_start_box1, available_end_box1)

	available_start_box2 := container.NewBorder(nil, nil, nil, available_start_reset2, available_start_arr[2])
	available_end_box2 := container.NewBorder(nil, nil, layout.NewSpacer(), available_end_reset2, available_end_arr[2])
	available_dates_box2 := container.NewAdaptiveGrid(2, available_start_box2, available_end_box2)

	available_start_box3 := container.NewBorder(nil, nil, nil, available_start_reset3, available_start_arr[3])
	available_end_box3 := container.NewBorder(nil, nil, layout.NewSpacer(), available_end_reset3, available_end_arr[3])
	available_dates_box3 := container.NewAdaptiveGrid(2, available_start_box3, available_end_box3)

	available_dates_box1.Hide()
	available_dates_box2.Hide()
	available_dates_box3.Hide()

	add_date_boxes := 1
	new_dates_arr = []*fyne.Container{available_dates_box, available_dates_box1, available_dates_box2, available_dates_box3}

	var available_vbox *fyne.Container
	add_dates_button := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "contentAdd"), func() {
		if available_start_arr[add_date_boxes-1].Text != "" && available_end_arr[add_date_boxes-1].Text != "" {
			if add_date_boxes < 4 {
				available_start_arr[add_date_boxes].Show()
				available_end_arr[add_date_boxes].Show()
				new_dates_arr[add_date_boxes].Show()
				add_date_boxes++
			}
		}
	})

	remove_dates_button := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "contentRemove"), func() {
		if add_date_boxes > 1 {
			available_start_arr[add_date_boxes-1].Text = ""
			available_start_arr[add_date_boxes-1].Hide()
			available_end_arr[add_date_boxes-1].Text = ""
			available_end_arr[add_date_boxes-1].Hide()
			new_dates_arr[add_date_boxes-1].Hide()
			add_date_boxes--
		}
	})

	available_vbox = container.NewVBox(container.NewAdaptiveGrid(2, add_dates_button, remove_dates_button), available_dates_box, available_dates_box1, available_dates_box2, available_dates_box3)
	available_border := container.NewBorder(nil, nil, nil, nil, available_vbox)

	change_dates := widget.NewButton("Off Dates", func() {
		derbnb_gif.Start()
		available_label := widget.NewLabel(fmt.Sprintf("Set off dates for SCID:\n%s", scid_entry.Text))
		available_label.Wrapping = fyne.TextWrapWord
		change_dates_cont := container.NewHSplit(container.NewBorder(available_label, nil, nil, nil, available_c), available_border)
		change_dates_cont.Offset = 0.47
		if imported {
			change_dates_cont.Offset = 0.4
		}
		confirm_border.Objects[4] = change_dates_cont
		confirm_action_label.SetText(fmt.Sprintf("Set off dates\n\nSCID: %s", viewing_address))
		confirm_action_int = 12
		confirm_border.Refresh()
		d.Window.SetContent(confirm_max)
	})

	set_location_button.Importance = widget.LowImportance
	list_button.Importance = widget.LowImportance
	remove_button.Importance = widget.LowImportance

	set_location_button.Hide()
	list_button.Hide()
	remove_button.Hide()
	confirm_request_button.Hide()
	cancel_request_button.Hide()
	release_button.Hide()
	change_price_button.Hide()

	cancel_booking_button.Hide()
	rate_booking_button.Hide()
	rate_renter_button.Hide()
	send_message.Hide()

	change_dates.Hide()

	trvl_button.Hide()
	request_button.Hide()
	message_listing.Hide()
	mint_prop.Hide()

	var property_add_info *widget.Button

	// renters list of rentals, requests and bookings as tree
	my_bookings.data = make(map[string][]string)
	booking_list = widget.NewTreeWithStrings(my_bookings.data)
	booking_list.OnSelected = func(uid widget.TreeNodeID) {
		split := strings.Split(uid, "   ")
		if len(split) >= 5 {
			confirm_action_scid = split[4]
			if stamp, err := strconv.ParseUint(split[1], 10, 64); err == nil {
				confirm_stamp = stamp
				viewing_address = getOwnerAddress(split[4])
				switch split[0] {
				case "Request:":
					rate_booking_button.Hide()
					if rpc.Wallet.IsConnected() {
						cancel_booking_button.Show()
						send_message.Show()
					}
				case "Booked:":
					cancel_booking_button.Hide()
					rate_booking_button.Hide()
					if rpc.Wallet.IsConnected() {
						send_message.Show()
					}
				case "Complete:":
					cancel_booking_button.Hide()
					if rpc.Wallet.IsConnected() {
						rate_booking_button.Show()
						send_message.Show()
					}
				default:
					cancel_booking_button.Hide()
					rate_booking_button.Hide()
					send_message.Hide()
				}
			}
		} else {
			confirm_stamp = 0
			confirm_action_scid = ""
			viewing_address = ""
			cancel_booking_button.Hide()
			rate_booking_button.Hide()
			send_message.Hide()
		}
	}

	booking_list.OnBranchClosed = func(uid widget.TreeNodeID) {
		confirm_stamp = 0
		confirm_action_scid = ""
		booking_list.UnselectAll()
		cancel_booking_button.Hide()
		rate_booking_button.Hide()
		send_message.Hide()
	}

	// owners list of properties, booking history tree for each scid
	my_properties.data = make(map[string][]string)
	property_list = widget.NewTreeWithStrings(my_properties.data)
	property_list.OnSelected = func(uid widget.TreeNodeID) {
		if len(uid) == 64 {
			confirm_stamp = 0
			confirm_dates = ""
			viewing_address = ""
			confirm_request_button.Hide()
			cancel_request_button.Hide()
			release_button.Hide()
			rate_renter_button.Hide()
			send_message.Hide()
			scid_entry.SetText(uid)
			return
		}

		split := strings.Split(uid, "   ")
		if len(split) > 2 {
			if stamp, err := strconv.ParseUint(split[1], 10, 64); err == nil {
				confirm_stamp = stamp
				viewing_address = split[4]
				confirm_dates = fmt.Sprintf("From: %s  -  To: %s", split[2], split[3])
				switch split[0] {
				case "Request:":
					if rpc.Wallet.IsConnected() {
						send_message.Show()
						confirm_request_button.Show()
						cancel_request_button.Show()
					}
					set_location_button.Hide()
					release_button.Hide()
					property_add_info.Hide()
					rate_renter_button.Hide()
				case "Booked:":
					if date, err := time.Parse(TIME_FORMAT, split[3]); err == nil {
						if date.Unix() < time.Now().UTC().Unix() {
							if rpc.Wallet.IsConnected() {
								release_button.Show()
							}
						} else {
							release_button.Hide()
						}
					}
					if rpc.Wallet.IsConnected() {
						send_message.Show()
					}
					set_location_button.Hide()
					confirm_request_button.Hide()
					cancel_request_button.Hide()
					rate_renter_button.Hide()
					property_add_info.Hide()
				case "Complete:":
					if rpc.Wallet.IsConnected() {
						rate_renter_button.Show()
						send_message.Show()
					}
					set_location_button.Hide()
					release_button.Hide()
					confirm_request_button.Hide()
					cancel_request_button.Hide()
					property_add_info.Hide()
				default:
					set_location_button.Hide()
					send_message.Hide()
					release_button.Hide()
					confirm_request_button.Hide()
					cancel_request_button.Hide()
					property_add_info.Hide()
					rate_renter_button.Hide()
				}
			}
		}
	}

	property_list.OnBranchOpened = func(uid widget.TreeNodeID) {
		property_list.Select(uid)
	}

	property_list.OnBranchClosed = func(uid widget.TreeNodeID) {
		confirm_stamp = 0
		confirm_dates = ""
		viewing_address = ""
		property_list.UnselectAll()
		release_button.Hide()
		set_location_button.Hide()
		confirm_request_button.Hide()
		cancel_request_button.Hide()
		cancel_request_button.Hide()
		rate_renter_button.Hide()
		send_message.Hide()
	}

	amt_cont := container.NewHScroll(container.NewAdaptiveGrid(2, price_entry, deposit_entry))
	amt_cont.SetMinSize(fyne.NewSize(330, 0))

	owner_entries := container.NewBorder(nil, nil, nil, amt_cont, scid_entry)
	owner_buttons := container.NewAdaptiveGrid(6,
		container.NewMax(change_dates),
		container.NewMax(confirm_request_button),
		container.NewMax(cancel_request_button),
		container.NewMax(release_button),
		container.NewMax(rate_renter_button),
		container.NewMax(change_price_button))

	control_box := container.NewVBox(owner_entries, owner_buttons)

	// bookings list control objects
	booking_scroll_up := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "arrowUp"), func() {
		booking_list.ScrollToTop()
	})

	booking_scroll_down := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "arrowDown"), func() {
		booking_list.ScrollToBottom()
	})

	booking_collapse_all := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "arrowDropDown"), func() {
		booking_list.UnselectAll()
		booking_list.CloseAllBranches()
	})

	booking_scroll_up.Importance = widget.LowImportance
	booking_scroll_down.Importance = widget.LowImportance
	booking_collapse_all.Importance = widget.LowImportance

	booking_list_control := container.NewBorder(nil, nil, booking_collapse_all, container.NewAdaptiveGrid(2, booking_scroll_up, booking_scroll_down), widget.NewLabel("Bookings"))

	// properties list control objects
	squared_symbol := "\u00B2"

	surface_entry := dwidget.NewDeroEntry("", 1, 0)
	surface_entry.SetPlaceHolder("Sq Meters:")
	surface_entry.AllowFloat = false
	surface_entry.Validator = validation.NewRegexp(`^[^0]\d{0,}$`, "Int required")
	meter_foot_entry := dwidget.NewDeroEntry("", 1, 0)
	meter_foot_entry.AllowFloat = false
	meter_foot_entry.SetPlaceHolder("Sq Feet:")
	meter_foot_button := widget.NewButton(fmt.Sprintf("Convert Foot%s to Meter%s", squared_symbol, squared_symbol), func() {
		if f, err := strconv.ParseFloat(meter_foot_entry.Text, 64); err == nil {
			meter := rpc.SqfootToSqMeter(f)
			if s := strconv.FormatFloat(meter, 'f', 0, 64); s != "" {
				surface_entry.SetText(s)
			}
		}
	})

	surface_cont := container.NewAdaptiveGrid(2, surface_entry, container.NewAdaptiveGrid(2, meter_foot_button, meter_foot_entry))

	styleOpts := []string{
		"House",
		"Apartment",
		"Penthouse",
		"Duplex",
		"Studio apartment/loft",
		"Boat",
		"Camper",
		"Container",
		"Earth home",
		"Farm",
		"Hotel",
		"Houseboat",
		"Tent",
		"Tree house",
	}

	style_entry := widget.NewSelect(styleOpts, nil)
	style_entry.PlaceHolder = "Style:"
	style_cont := container.NewVBox(style_entry)

	num_bedrooms_entry := dwidget.NewDeroEntry("", 1, 0)
	num_bedrooms_entry.SetPlaceHolder("Number of bedrooms:")
	num_bedrooms_entry.AllowFloat = false
	num_bedrooms_entry.Validator = validation.NewRegexp(`^[^0]\d{0,}$`, "Int required")
	num_bedrooms_cont := container.NewVBox(num_bedrooms_entry)

	num_guests_entry := dwidget.NewDeroEntry("", 1, 0)
	num_guests_entry.SetPlaceHolder("Max guests:")
	num_guests_entry.AllowFloat = false
	num_guests_entry.Validator = validation.NewRegexp(`^[^0]\d{0,}$`, "Int required")
	num_guests_cont := container.NewVBox(num_guests_entry)

	photo_entry1 := widget.NewEntry()
	photo_entry2 := widget.NewEntry()
	photo_entry3 := widget.NewEntry()
	photo_entry4 := widget.NewEntry()
	photo_entry5 := widget.NewEntry()
	photo_entry6 := widget.NewEntry()
	photo_entry7 := widget.NewEntry()
	photo_entry8 := widget.NewEntry()
	photo_entry9 := widget.NewEntry()
	photo_entry1.SetPlaceHolder("Photo #1:")
	photo_entry2.SetPlaceHolder("Photo #2:")
	photo_entry3.SetPlaceHolder("Photo #3:")
	photo_entry4.SetPlaceHolder("Photo #4:")
	photo_entry5.SetPlaceHolder("Photo #5:")
	photo_entry6.SetPlaceHolder("Photo #6:")
	photo_entry7.SetPlaceHolder("Photo #7:")
	photo_entry8.SetPlaceHolder("Photo #8:")
	photo_entry9.SetPlaceHolder("Photo #9:")

	photo_entry_cont := container.NewVBox(
		photo_entry1,
		photo_entry2,
		photo_entry3,
		photo_entry4,
		photo_entry5,
		photo_entry6,
		photo_entry7,
		photo_entry8,
		photo_entry9)

	prop_descp_entry := widget.NewMultiLineEntry()
	prop_descp_entry.SetPlaceHolder("Description:")
	prop_descp_entry.Validator = validation.NewRegexp(`^\w{1,}`, "String required")
	prop_descp_entry.Wrapping = fyne.TextWrapWord
	prop_descp_cont := container.NewVBox(prop_descp_entry)

	cleaning_fee_entry := dwidget.NewDeroEntry("", 1, 0)
	cleaning_fee_entry.SetPlaceHolder("Cleaning fee:")
	cleaning_fee_entry.AllowFloat = false
	cleaning_fee_entry.Validator = validation.NewRegexp(`^[^0]\d{0,}$`, "Int required")
	cleaning_fee_cont := container.NewVBox(cleaning_fee_entry)

	min_stay_entry := dwidget.NewDeroEntry("", 1, 0)
	min_stay_entry.SetPlaceHolder("Minimum stay:")
	min_stay_entry.AllowFloat = false
	min_stay_entry.Validator = validation.NewRegexp(`^[^0]\d{0,}$`, "Int required")
	min_stay_cont := container.NewVBox(min_stay_entry)

	max_stay_entry := dwidget.NewDeroEntry("", 1, 0)
	max_stay_entry.SetPlaceHolder("Maximum stay:")
	max_stay_entry.AllowFloat = false
	max_stay_entry.Validator = validation.NewRegexp(`^[^0]\d{0,}$`, "Int required")
	max_stay_cont := container.NewVBox(max_stay_entry)

	prop_name_entry := widget.NewEntry()
	prop_name_entry.SetPlaceHolder("Property name:")
	prop_name_entry.Validator = validation.NewRegexp(`^\w{1,}`, "String required")
	prop_name_cont := container.NewVBox(prop_name_entry)

	shareOpts := []string{"Entire place", "A room", "A shared room"}
	prop_share_entry := widget.NewSelect(shareOpts, nil)
	prop_share_entry.PlaceHolder = "Style:"
	prop_share_cont := container.NewVBox(prop_share_entry)

	prop_rules_entry := widget.NewMultiLineEntry()
	prop_rules_entry.SetPlaceHolder("Property rules:")
	prop_rules_entry.Validator = validation.NewRegexp(`^\w{1,}`, "String required")
	prop_rules_cont := container.NewVBox(prop_rules_entry)

	bathrooms_entry := dwidget.NewDeroEntry("", 1, 0)
	bathrooms_entry.SetPlaceHolder("Bathrooms:")
	bathrooms_entry.AllowFloat = false
	bathrooms_entry.Validator = validation.NewRegexp(`^[^0]\d{0,}$`, "Int required")
	bathrooms_cont := container.NewVBox(bathrooms_entry)

	petOpts := []string{"No", "Yes"}
	pet_radio := widget.NewRadioGroup(petOpts, nil)
	pet_radio.Horizontal = true
	pet_radio.SetSelected("No")
	pet_cont := container.NewVBox(pet_radio)

	metadata_label_arr = []string{
		fmt.Sprintf("Surface (m%s)", squared_symbol),
		"Property Style",
		"Bedrooms",
		"Max Guests",
		"Photos",
		"Describe the Property",
		"Cleaning Fee",
		"Minimum Stay",
		"Maximum Stay",
		"Name",
		"Share",
		"Rules",
		"Bathrooms",
		"Pets",
		"Wifi",
		"TV",
		"Kitchen",
		"Washer",
		"Free Parking",
		"Air conditioner",
		"Workspace",
		"Pool Access",
		"Hot Tub",
		"BBQ",
		"Outdoor Dining",
		"Fire Pit",
		"Games Room",
		"Exercise Equipment",
		"Lake Access",
		"Beach Access",
		"Smoke Alarm",
		"Fire Extinguisher",
	}

	metadata_entry_arr = []*fyne.Container{
		surface_cont,
		style_cont,
		num_bedrooms_cont,
		num_guests_cont,
		photo_entry_cont,
		prop_descp_cont,
		cleaning_fee_cont,
		min_stay_cont,
		max_stay_cont,
		prop_name_cont,
		prop_share_cont,
		prop_rules_cont,
		bathrooms_cont,
		pet_cont,
	}

	for i := 0; i < 18; i++ {
		metadata_entry_arr = append(metadata_entry_arr, placeAmenityObjects())
	}

	property_add_info = widget.NewButtonWithIcon("Edit", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "documentCreate"), func() {
		derbnb_gif.Start()
		confirm_border.Objects[4] = container.NewVScroll(container.NewVBox(layout.NewSpacer(), confirm_action_label, layout.NewSpacer(), placeMetadataObjects(metadata_label_arr, metadata_entry_arr)))
		confirm_action_label.SetText(fmt.Sprintf("Set property info\n\nSCID: %s", scid_entry.Text))
		confirm_action_int = 14
		confirm_border.Refresh()
		d.Window.SetContent(confirm_max)
	})
	property_add_info.Hide()

	property_scroll_up := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "arrowUp"), func() {
		property_list.ScrollToTop()
	})

	property_scroll_down := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "arrowDown"), func() {
		property_list.ScrollToBottom()
	})

	property_collapse_all := widget.NewButtonWithIcon("", fyne.Theme.Icon(fyne.CurrentApp().Settings().Theme(), "arrowDropDown"), func() {
		property_list.UnselectAll()
		property_list.CloseAllBranches()
		scid_entry.SetText("")
	})

	property_add_info.Importance = widget.LowImportance
	property_scroll_up.Importance = widget.LowImportance
	property_scroll_down.Importance = widget.LowImportance
	property_collapse_all.Importance = widget.LowImportance

	property_list_control := container.NewBorder(
		nil,
		nil,
		property_collapse_all,
		container.NewAdaptiveGrid(2, property_scroll_up, property_scroll_down),
		container.NewHBox(widget.NewLabel("Properties"), set_location_button, list_button, remove_button, property_add_info))

	user_info := container.NewVSplit(
		container.NewBorder(booking_list_control, container.NewAdaptiveGrid(4, container.NewMax(cancel_booking_button), container.NewMax(rate_booking_button), container.NewMax(send_message), container.NewMax(trvl_button)), nil, nil, booking_list),
		container.NewBorder(property_list_control, nil, nil, nil, property_list))

	layout1 := container.NewBorder(dates_box, container.NewAdaptiveGrid(3, message_listing, request_button, layout.NewSpacer()), nil, nil, layout1_split)
	layout2 := container.NewBorder(nil, control_box, nil, nil, user_info)

	tab_alpha := canvas.NewRectangle(color.RGBA{0, 0, 0, 120})
	if bundle.AppColor == color.White {
		tab_alpha = canvas.NewRectangle(color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x55})
	}

	tabs = container.NewAppTabs(
		container.NewTabItem("Properties", container.NewMax(tab_alpha, layout1)),
		container.NewTabItem("Profile", container.NewMax(tab_alpha, layout2)))

	if !imported {
		tabs.Append(container.NewTabItem("Log", rpc.SessionLog()))
	}

	tabs.SetTabLocation(container.TabLocationBottom)
	tabs.OnSelected = func(ti *container.TabItem) {
		switch ti.Text {
		case "Properties":
			viewing_scid = ""
			viewing_address = ""
			request_button.Hide()
			message_listing.Hide()
			send_message.Hide()
			listing_label.SetText("")
			listings_list.UnselectAll()
			my_bookings.RLock()
			booking_list.Refresh()
			my_bookings.RUnlock()
			property_list.Refresh()
		default:

		}
	}

	if imported {
		tab_bottom := canvas.NewRectangle(color.RGBA{0, 0, 0, 180})
		alpha := canvas.NewRectangle(color.RGBA{0, 0, 0, 150})
		if bundle.AppColor == color.White {
			alpha = canvas.NewRectangle(color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xaa})
		}

		tab_bottom.SetMinSize(fyne.NewSize(157, 40))
		tab_bottom_box := container.NewHBox(container.NewMax(tab_bottom, alpha), layout.NewSpacer())
		tab_bottom_bar := container.NewVBox(layout.NewSpacer(), tab_bottom_box)

		max = container.NewMax(tab_bottom_bar, tabs)
	} else {
		tag := "DerBnb"
		connect_box = dwidget.NewHorizontalEntries(tag, 1)
		connect_box.Button.OnTapped = func() {
			rpc.GetAddress(tag)
			rpc.Ping()
			if rpc.Daemon.IsConnected() && !menu.Gnomes.IsInitialized() && !menu.Gnomes.Start {
				property_filter = menu.FetchFilters("derbnb_filter")
				filters := BnbSearchFilter()
				go menu.StartGnomon(tag, menu.Gnomes.DBType, filters, 0, 0, nil)
			}
		}

		connect_box.Disconnect.OnChanged = func(b bool) {
			if !b {
				menu.Gnomes.Stop(tag)
			}
		}

		config := menu.ReadDreamsConfig(tag)
		connect_box.AddDaemonOptions(config.Daemon)

		connect_box.Container.Objects[0].(*fyne.Container).Add(menu.StartIndicators())

		max = container.NewMax(tabs, container.NewVBox(layout.NewSpacer(), connect_box.Container))
	}

	property_photos.data = make(map[string][]string)

	// Two routines, if imported exit with dreams.CloseDapp()
	go func() {
		time.Sleep(3 * time.Second)
		if imported {
			for {
				select {
				case <-d.Receive():
					if !rpc.Wallet.IsConnected() || !rpc.Daemon.IsConnected() {
						list_button.Hide()
						remove_button.Hide()
						confirm_request_button.Hide()
						cancel_request_button.Hide()
						release_button.Hide()
						rate_renter_button.Hide()
						change_price_button.Hide()

						cancel_booking_button.Hide()
						rate_booking_button.Hide()
						send_message.Hide()

						change_dates.Hide()
						property_add_info.Hide()

						trvl_button.Hide()
						request_button.Hide()
						message_listing.Hide()
						mint_prop.Hide()

						image_box.Objects[0] = &image
						image_box.Refresh()

						listing_label.SetText("")
						d.WorkDone()
						continue
					}

					trvl_button.Show()
					mint_prop.Show()

					go GetProperties()
					profileValidators()
					viewingValidators()

					d.WorkDone()
				case <-d.CloseDapp():
					logger.Println("[DerBnb] Done")
					return
				}
			}
		} else {
			i := 0
			for !menu.ClosingApps() {
				if !rpc.Wallet.IsConnected() || !rpc.Daemon.IsConnected() {
					list_button.Hide()
					remove_button.Hide()
					confirm_request_button.Hide()
					cancel_request_button.Hide()
					release_button.Hide()
					rate_renter_button.Hide()
					change_price_button.Hide()

					cancel_booking_button.Hide()
					rate_booking_button.Hide()
					send_message.Hide()

					change_dates.Hide()
					property_add_info.Hide()

					trvl_button.Hide()
					request_button.Hide()
					message_listing.Hide()
					mint_prop.Hide()

					image_box.Objects[0] = &image
					image_box.Refresh()

					listing_label.SetText("")
					continue
				}

				trvl_button.Show()
				mint_prop.Show()

				if imported && i == 3 {
					i = 0
					GetProperties()
				}

				profileValidators()
				viewingValidators()

				i++

				time.Sleep(time.Second)
			}
		}
	}()

	viewingValidators = func() {
		if len(viewing_scid) != 64 {
			request_button.Hide()
			message_listing.Hide()
			image_box.Objects[0] = canvas.NewImageFromImage(nil)
			image_box.Refresh()
		}
	}

	profileValidators = func() {
		if scid_entry.Validate() == nil && rpc.Wallet.IsConnected() {
			if haveProperty(scid_entry.Text) {
				change_dates.Show()
				remove_button.Show()
				set_location_button.Hide()
				list_button.Hide()
				if price_entry.Validate() == nil && deposit_entry.Validate() == nil {
					change_price_button.Show()
				} else {
					change_price_button.Hide()
				}
			} else {
				go func() {
					remove_button.Hide()
					change_dates.Hide()
					change_price_button.Hide()
					set_location_button.Hide()
					if checkAssetContract(scid_entry.Text) == TOKEN_CONTRACT {
						if rpc.TokenBalance(scid_entry.Text) == 1 {
							if city, country := getLocation(scid_entry.Text); city == "" && country == "" {
								set_location_button.Show()
							} else {
								property_add_info.Show()
								if price_entry.Validate() == nil && deposit_entry.Validate() == nil {
									list_button.Show()
								}
							}
						}
					}
				}()
			}
		} else {
			list_button.Hide()
			remove_button.Hide()
			confirm_request_button.Hide()
			cancel_request_button.Hide()
			release_button.Hide()
			change_price_button.Hide()
			change_dates.Hide()
			property_add_info.Hide()
			set_location_button.Hide()
		}

		if price_entry.Validate() == nil && deposit_entry.Validate() == nil {
			if scid_entry.Validate() == nil && rpc.Wallet.IsConnected() {
				if !haveProperty(scid_entry.Text) {
					list_button.Show()
				} else {
					if checkAssetContract(scid_entry.Text) == TOKEN_CONTRACT {
						if getOwnerAddress(scid_entry.Text) == rpc.Wallet.Address {
							change_price_button.Show()
						}
					}
				}
			}
		} else {
			list_button.Hide()
			change_price_button.Hide()
		}
	}

	return max
}

func placeAmenityObjects() *fyne.Container {
	opts := []string{"No", "Yes"}
	amenity := widget.NewRadioGroup(opts, nil)
	amenity.Horizontal = true
	amenity.SetSelected("No")

	return container.NewVBox(amenity)

}

// Places metadata labels with entry objects into container
func placeMetadataObjects(label []string, entry []*fyne.Container) fyne.CanvasObject {
	metadata_form := []*widget.FormItem{}
	for i := range label {
		metadata_form = append(metadata_form, widget.NewFormItem(label[i], entry[i]))

	}

	return widget.NewForm(metadata_form...)
}

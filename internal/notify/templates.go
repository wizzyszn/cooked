package notify

import (
	"fmt"
	"strings"
)

const (
	TemplateSignIn        = "sign_in"
	TemplateVerifyEmail   = "verify_email"
	TemplateOnboarded     = "onboarded"
	TemplateForgotPassOtp = "forgot_otp"
)

type Rendered struct {
	Title string
	Body  string
	HTML  string
}

func Render(template string, payload map[string]any) *Rendered {
	switch template {
	case TemplateSignIn:
		name := payloadString(payload, "name", "there")
		return &Rendered{
			Title: "New sign-in to Cooked",
			Body:  fmt.Sprintf("Hi %s, someone just signed in to your Cooked account. If this wasn't you, reset your password.", name),
			HTML: emailHTML(
				"New sign-in to Cooked",
				fmt.Sprintf("Hi %s,", htmlEscape(name)),
				"Someone just signed in to your Cooked account. If this was you, you can ignore this email.",
				"",
				"",
			),
		}
	case TemplateVerifyEmail:
		name := payloadString(payload, "name", "there")
		link := payloadString(payload, "verify_url", "")
		return &Rendered{
			Title: "Confirm your Cooked email",
			Body:  fmt.Sprintf("Hi %s, confirm your email by opening this link: %s", name, link),
			HTML: emailHTML(
				"Confirm your email",
				fmt.Sprintf("Hi %s,", htmlEscape(name)),
				"Thanks for joining Cooked. Confirm your email to start sharing recipes.",
				htmlEscape(link),
				"Confirm email",
			),
		}
	case TemplateOnboarded:
		name := payloadString(payload, "name", "there")
		return &Rendered{
			Title: "Welcome to Cooked",
			Body:  fmt.Sprintf("Hi %s, your Cooked account is ready. Time to cook something good.", name),
			HTML: emailHTML(
				"Welcome to Cooked",
				fmt.Sprintf("Hi %s,", htmlEscape(name)),
				"Your account is ready. Time to cook something good.",
				"",
				"",
			),
		}
	case TemplateForgotPassOtp:
		name := payloadString(payload, "name", "there")
		otp := payloadString(payload, "otp", "")
		return &Rendered{
			Title: "Reset your Cooked password",
			Body:  fmt.Sprintf("Hi %s, your password reset code is %s. It expires in 30 minutes.", name, otp),
			HTML: emailHTML(
				"Reset your password",
				fmt.Sprintf("Hi %s,", htmlEscape(name)),
				fmt.Sprintf("Use this code to reset your password. It expires in 30 minutes: %s", otp),
				"",
				"",
			),
		}
	default:
		return nil
	}
}

func payloadString(payload map[string]any, key, fallback string) string {
	if payload == nil {
		return fallback
	}
	v, ok := payload[key]
	if !ok || v == nil {
		return fallback
	}
	s, ok := v.(string)
	if !ok {
		return fallback
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	return s
}

func emailHTML(heading, greeting, body, href, cta string) string {
	button := ""
	if href != "" && cta != "" {
		button = fmt.Sprintf(
			`<p style="margin:28px 0"><a href="%s" style="background:#111;color:#fff;text-decoration:none;padding:12px 20px;border-radius:6px;display:inline-block">%s</a></p>
<p style="color:#666;font-size:12px;word-break:break-all">Or paste this link into your browser:<br>%s</p>`,
			href, htmlEscape(cta), href,
		)
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:-apple-system,BlinkMacSystemFont,Segoe UI,sans-serif;line-height:1.5;color:#111;max-width:560px;margin:0 auto;padding:24px">
  <h1 style="font-size:22px;margin:0 0 16px">%s</h1>
  <p>%s</p>
  <p>%s</p>
  %s
  <p style="color:#888;font-size:12px;margin-top:32px">Cooked</p>
</body>
</html>`, htmlEscape(heading), greeting, htmlEscape(body), button)
}

func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

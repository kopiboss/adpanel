package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"adpanel/config"
	"adpanel/middleware"
	"adpanel/models"
	"adpanel/services"
)

func ShowLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "auth/login.html", gin.H{
		"title": "Login",
	})
}

func HandleLogin(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")

	// Check super admin
	if email == config.App.AdminEmail {
		if err := bcrypt.CompareHashAndPassword([]byte(""), []byte(password)); err != nil {
			// Admin password is plain from env, compare directly
		}
		if password == config.App.AdminPassword {
			session := sessions.Default(c)
			session.Set(middleware.SessionIsAdmin, true)
			session.Set(middleware.SessionUserRole, "superadmin")
			session.Save()
			c.Redirect(http.StatusFound, "/admin")
			return
		}
		c.HTML(http.StatusOK, "auth/login.html", gin.H{
			"error": "Email atau password salah",
			"title": "Login",
		})
		return
	}

	user, err := models.GetUserByEmail(email)
	if err != nil {
		c.HTML(http.StatusOK, "auth/login.html", gin.H{
			"error": "Email atau password salah",
			"title": "Login",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		c.HTML(http.StatusOK, "auth/login.html", gin.H{
			"error": "Email atau password salah",
			"title": "Login",
		})
		return
	}

	if user.Status == "pending" {
		c.Redirect(http.StatusFound, "/pending")
		return
	}

	if user.Status == "suspended" {
		c.HTML(http.StatusOK, "auth/login.html", gin.H{
			"error": "Akun Anda telah disuspend. Hubungi admin.",
			"title": "Login",
		})
		return
	}

	session := sessions.Default(c)
	session.Set(middleware.SessionUserID, user.ID)
	session.Set(middleware.SessionUserRole, user.Role)
	session.Save()

	c.Redirect(http.StatusFound, "/dashboard")
}

func HandleLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/login")
}

func ShowRegister(c *gin.Context) {
	c.HTML(http.StatusOK, "auth/register.html", gin.H{
		"title": "Daftar",
	})
}

func HandleRegister(c *gin.Context) {
	name := c.PostForm("name")
	email := c.PostForm("email")
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")

	if name == "" || email == "" || password == "" {
		c.HTML(http.StatusOK, "auth/register.html", gin.H{
			"error": "Semua field wajib diisi",
			"title": "Daftar",
		})
		return
	}

	if password != confirmPassword {
		c.HTML(http.StatusOK, "auth/register.html", gin.H{
			"error": "Password tidak cocok",
			"title": "Daftar",
		})
		return
	}

	if len(password) < 8 {
		c.HTML(http.StatusOK, "auth/register.html", gin.H{
			"error": "Password minimal 8 karakter",
			"title": "Daftar",
		})
		return
	}

	// Check email exists
	if _, err := models.GetUserByEmail(email); err == nil {
		c.HTML(http.StatusOK, "auth/register.html", gin.H{
			"error": "Email sudah terdaftar",
			"title": "Daftar",
		})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.HTML(http.StatusOK, "auth/register.html", gin.H{
			"error": "Terjadi kesalahan, coba lagi",
			"title": "Daftar",
		})
		return
	}

	user := &models.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
		Role:         "user",
		Status:       "pending",
	}

	userID, err := models.CreateUser(user)
	if err != nil {
		c.HTML(http.StatusOK, "auth/register.html", gin.H{
			"error": "Gagal mendaftar, coba lagi",
			"title": "Daftar",
		})
		return
	}

	user.ID = userID

	// Notify admin via Telegram
	if services.Bot != nil {
		go services.Bot.NotifyNewUser(user)
	}

	c.Redirect(http.StatusFound, "/pending")
}

func ShowPending(c *gin.Context) {
	c.HTML(http.StatusOK, "auth/pending.html", gin.H{
		"title": "Menunggu Persetujuan",
	})
}

func GoogleOAuthStart(c *gin.Context) {
	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)

	session := sessions.Default(c)
	session.Set("oauth_state", state)
	session.Save()

	url := services.GoogleAuthURL(state)
	if url == "" {
		c.HTML(http.StatusOK, "auth/login.html", gin.H{
			"error": "Google OAuth belum dikonfigurasi",
			"title": "Login",
		})
		return
	}

	c.Redirect(http.StatusFound, url)
}

func GoogleOAuthCallback(c *gin.Context) {
	session := sessions.Default(c)
	expectedState, _ := session.Get("oauth_state").(string)

	if state := c.Query("state"); state != expectedState {
		c.HTML(http.StatusOK, "auth/login.html", gin.H{
			"error": "Invalid OAuth state",
			"title": "Login",
		})
		return
	}

	code := c.Query("code")
	token, err := services.GoogleExchangeCode(c.Request.Context(), code)
	if err != nil {
		c.HTML(http.StatusOK, "auth/login.html", gin.H{
			"error": "Gagal autentikasi Google",
			"title": "Login",
		})
		return
	}

	googleUser, err := services.GetGoogleUserInfo(token)
	if err != nil {
		c.HTML(http.StatusOK, "auth/login.html", gin.H{
			"error": "Gagal mendapatkan info user Google",
			"title": "Login",
		})
		return
	}

	// Try to find existing user by Google ID or email
	user, err := models.GetUserByGoogleID(googleUser.ID)
	if err != nil {
		// Try by email
		user, err = models.GetUserByEmail(googleUser.Email)
		if err != nil {
			// Create new user
			newUser := &models.User{
				Name:     googleUser.Name,
				Email:    googleUser.Email,
				GoogleID: googleUser.ID,
				Role:     "user",
				Status:   "pending",
			}
			userID, err := models.CreateUser(newUser)
			if err != nil {
				c.HTML(http.StatusOK, "auth/login.html", gin.H{
					"error": "Gagal membuat akun",
					"title": "Login",
				})
				return
			}
			newUser.ID = userID
			go services.Bot.NotifyNewUser(newUser)
			c.Redirect(http.StatusFound, "/pending")
			return
		}
		// Link Google ID to existing user
		_ = models.UpdateUserGoogleID(user.ID, googleUser.ID)
	}

	if user.Status == "pending" {
		c.Redirect(http.StatusFound, "/pending")
		return
	}

	if user.Status == "suspended" {
		c.HTML(http.StatusOK, "auth/login.html", gin.H{
			"error": "Akun Anda telah disuspend",
			"title": "Login",
		})
		return
	}

	session.Set(middleware.SessionUserID, user.ID)
	session.Set(middleware.SessionUserRole, user.Role)
	session.Delete("oauth_state")
	session.Save()

	c.Redirect(http.StatusFound, "/dashboard")
}

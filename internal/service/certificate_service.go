package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"x-ui/internal/model"

	"gorm.io/gorm"
)

type CertificateService struct {
	db *gorm.DB
}

func NewCertificateService(db *gorm.DB) *CertificateService {
	return &CertificateService{db: db}
}

type CertificateInfo struct {
	Domain      string    `json:"domain"`
	CertFile    string    `json:"cert_file,omitempty"`
	KeyFile     string    `json:"key_file,omitempty"`
	Issuer      string    `json:"issuer,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsValid     bool      `json:"is_valid"`
	DaysLeft    int       `json:"days_left,omitempty"`
	AutoRenew   bool      `json:"auto_renew"`
	LastChecked *time.Time `json:"last_checked,omitempty"`
}

func (s *CertificateService) SearchDomain(ctx context.Context, query string) ([]CertificateInfo, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	var certs []model.DomainCertificate
	searchPattern := "%" + strings.ToLower(query) + "%"
	
	if err := s.db.WithContext(ctx).
		Where("LOWER(domain) LIKE ?", searchPattern).
		Find(&certs).Error; err != nil {
		return nil, err
	}

	results := make([]CertificateInfo, 0, len(certs))
	for _, cert := range certs {
		info := s.certToInfo(&cert)
		results = append(results, info)
	}

	return results, nil
}

func (s *CertificateService) GetCertificateByDomain(ctx context.Context, domain string) (*CertificateInfo, error) {
	var cert model.DomainCertificate
	if err := s.db.WithContext(ctx).
		Where("domain = ?", domain).
		First(&cert).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("certificate for domain %s not found", domain)
		}
		return nil, err
	}

	info := s.certToInfo(&cert)
	return &info, nil
}

func (s *CertificateService) certToInfo(cert *model.DomainCertificate) CertificateInfo {
	info := CertificateInfo{
		Domain:      cert.Domain,
		CertFile:    cert.CertFile,
		KeyFile:     cert.KeyFile,
		Issuer:      cert.Issuer,
		ExpiresAt:   cert.ExpiresAt,
		AutoRenew:   cert.AutoRenew,
		LastChecked: cert.LastChecked,
	}

	if cert.ExpiresAt != nil {
		now := time.Now()
		daysLeft := int(cert.ExpiresAt.Sub(now).Hours() / 24)
		info.DaysLeft = daysLeft
		info.IsValid = cert.ExpiresAt.After(now)
	} else {
		info.IsValid = false
	}

	return info
}

func (s *CertificateService) LoadCertificateFromFile(certFile, keyFile string) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}
	return &cert, nil
}

func (s *CertificateService) ParseCertificate(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

func (s *CertificateService) CheckCertificate(ctx context.Context, domain string) (*CertificateInfo, error) {
	var cert model.DomainCertificate
	if err := s.db.WithContext(ctx).
		Where("domain = ?", domain).
		First(&cert).Error; err != nil {
		return nil, err
	}

	now := time.Now()
	cert.LastChecked = &now

	if cert.CertFile != "" && cert.KeyFile != "" {
		if err := s.updateCertificateInfoFromFile(&cert); err != nil {
			return nil, err
		}
	} else if len(cert.CertContent) > 0 {
		if err := s.updateCertificateInfoFromContent(&cert); err != nil {
			return nil, err
		}
	}

	if err := s.db.WithContext(ctx).Save(&cert).Error; err != nil {
		return nil, err
	}

	info := s.certToInfo(&cert)
	return &info, nil
}

func (s *CertificateService) updateCertificateInfoFromFile(cert *model.DomainCertificate) error {
	if _, err := os.Stat(cert.CertFile); os.IsNotExist(err) {
		return fmt.Errorf("certificate file not found: %s", cert.CertFile)
	}

	certPEM, err := os.ReadFile(cert.CertFile)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %w", err)
	}

	return s.updateCertificateInfo(cert, certPEM)
}

func (s *CertificateService) updateCertificateInfoFromContent(cert *model.DomainCertificate) error {
	return s.updateCertificateInfo(cert, cert.CertContent)
}

func (s *CertificateService) updateCertificateInfo(cert *model.DomainCertificate, certPEM []byte) error {
	parsedCert, err := s.ParseCertificate(certPEM)
	if err != nil {
		return err
	}

	cert.Issuer = parsedCert.Issuer.String()
	expiresAt := parsedCert.NotAfter
	cert.ExpiresAt = &expiresAt

	return nil
}

func (s *CertificateService) AddOrUpdateCertificate(ctx context.Context, domain, certFile, keyFile string, certContent, keyContent []byte, autoRenew bool) error {
	var cert model.DomainCertificate
	err := s.db.WithContext(ctx).
		Where("domain = ?", domain).
		First(&cert).Error

	isNew := err == gorm.ErrRecordNotFound
	if err != nil && !isNew {
		return err
	}

	cert.Domain = domain
	cert.CertFile = certFile
	cert.KeyFile = keyFile
	cert.AutoRenew = autoRenew

	if len(certContent) > 0 {
		cert.CertContent = certContent
	}
	if len(keyContent) > 0 {
		cert.KeyContent = keyContent
	}

	if certFile != "" && keyFile != "" {
		if err := s.updateCertificateInfoFromFile(&cert); err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}
	} else if len(certContent) > 0 {
		if err := s.updateCertificateInfoFromContent(&cert); err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}
	}

	if isNew {
		return s.db.WithContext(ctx).Create(&cert).Error
	}
	return s.db.WithContext(ctx).Save(&cert).Error
}


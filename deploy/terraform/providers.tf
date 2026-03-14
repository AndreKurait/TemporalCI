terraform {
  required_version = ">= 1.5"

  backend "s3" {
    bucket = "temporalci-tfstate-969644818851"
    key    = "terraform.tfstate"
    region = "us-east-1"
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

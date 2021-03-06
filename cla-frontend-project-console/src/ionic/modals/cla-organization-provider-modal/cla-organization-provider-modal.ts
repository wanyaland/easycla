// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

import { Component } from '@angular/core';
import { NavParams, ViewController, IonicPage, Events } from 'ionic-angular';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ClaService } from '../../services/cla.service';
import { PlatformLocation } from '@angular/common';

@IonicPage({
  segment: 'cla-organization-provider-modal'
})
@Component({
  selector: 'cla-organization-provider-modal',
  templateUrl: 'cla-organization-provider-modal.html'
})
export class ClaOrganizationProviderModal {
  form: FormGroup;
  submitAttempt: boolean = false;
  currentlySubmitting: boolean = false;
  responseErrors: string[] = [];
  claProjectId: any;
  showErrorMsg: boolean;
  loading: boolean;
  sfdcProjectId: string;

  constructor(
    public navParams: NavParams,
    public viewCtrl: ViewController,
    private formBuilder: FormBuilder,
    public claService: ClaService,
    public events: Events,
    private location: PlatformLocation
  ) {
    this.claProjectId = this.navParams.get('claProjectId');
    this.sfdcProjectId = this.navParams.get('sfdcProjectId');
    this.form = formBuilder.group({
      orgName: ['', Validators.compose([Validators.required]) /*, this.urlCheck.bind(this)*/]
    });
    this.location.onPopState(() => {
      this.viewCtrl.dismiss(false);
    });
    events.subscribe('modal:close', () => {
      this.dismiss();
    });
  }

  submit() {
    this.submitAttempt = true;
    this.currentlySubmitting = true;
    if (!this.form.valid) {
      this.currentlySubmitting = false;
      return;
    }
    this.checkGitOrganisationName();
  }

  checkGitOrganisationName() {
    this.loading = true;
    this.showErrorMsg = false;
    let trimName = this.form.value.orgName.trim();
    this.claService.testGitHubOrganization(trimName).subscribe((res: any) => {
      this.loading = false;
      if (res.status === 200) {
        this.postClaGithubOrganization();
      }
    }, (err: any) => {
      this.loading = false;
      if (!err.ok) {
        this.showErrorMsg = true;
      }
    })
  }

  postClaGithubOrganization() {
    const organization = {
      autoEnabled: false,
      branchProtectionEnabled: false,
      organizationName: this.form.value.orgName.trim()
    };
    this.claService.postGithubOrganization(this.sfdcProjectId, organization).subscribe(
      () => {
        this.dismiss(true);
      },
      (error) => {
        if (error._body) {
          this.responseErrors = [];
          this.responseErrors.push(JSON.parse(error._body).Message);
        }
      }
    );
  }

  dismiss(data = false) {
    this.viewCtrl.dismiss(data);
  }

  clearError(event) {
    this.showErrorMsg = false;
    this.responseErrors = [];
    this.submitAttempt = false;
  }
}

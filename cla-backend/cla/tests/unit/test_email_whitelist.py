# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

from unittest.mock import patch,Mock,MagicMock

import pytest

from cla.models.dynamo_models import User,Signature,UserModel


EMAIL_WHITELISTS = ['abc@att.com','foo@bar.com','harold@cla.com']
DOMAIN_WHITELISTS = ['*.att.com', '*att.com']


@pytest.fixture()
def create_user():
    """ Mock user instance """
    with patch.object(User,'__init__',lambda self: None):
        user = User()
        user.model = UserModel()
        yield user


def test_is_email_whitelisted(create_user):
    """
    Test function to check list if one of the users addresses are whitelisted against a ccla signature
    """
    create_user.set_user_email('abc@att.com')
    Signature = Mock()
    Signature.get_email_whitelist.return_value = EMAIL_WHITELISTS
    Signature.get_domain_whitelist.return_value = DOMAIN_WHITELISTS
    assert create_user.is_whitelisted(Signature) == True

def test_email_is_not_whitelisted(create_user):
    """
    Test email that is neither in email whitelist and domain whitelist
    """
    create_user.set_user_email('pee@nacho.com')
    Signature = Mock()
    Signature.get_email_whitelist.return_value = EMAIL_WHITELISTS
    Signature.get_domain_whitelist.return_value = DOMAIN_WHITELISTS
    assert create_user.is_whitelisted(Signature) == False

def test_email_is_domain_whitelisted(create_user):
    """
    Test email that is domain whitelisted
    """
    create_user.set_user_email('foo@att.com')
    Signature = Mock()
    Signature.get_email_whitelist.return_value = EMAIL_WHITELISTS
    Signature.get_domain_whitelist.return_value = DOMAIN_WHITELISTS
    assert create_user.is_whitelisted(Signature) == True

def test_email_is_not_domain_whitelisted(create_user):
    """
    Test email not in email whitelist and domain whitelist
    """
    create_user.set_user_email('sorry@gmail.com')
    Signature = Mock()
    Signature.get_email_whitelist.return_value = EMAIL_WHITELISTS
    Signature.get_domain_whitelist.return_value = DOMAIN_WHITELISTS
    assert create_user.is_whitelisted(Signature) == False


def test_email_with_subdomain_in_domain_whitelist(create_user):
    """
    Test valid subdomain instance
    """
    create_user.set_user_email('foo@help.att.com')
    Signature = Mock()
    Signature.get_email_whitelist.return_value = EMAIL_WHITELISTS
    Signature.get_domain_whitelist.return_value = DOMAIN_WHITELISTS
    assert create_user.is_whitelisted(Signature) == True








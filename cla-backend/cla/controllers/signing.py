"""
Controller related to the signed callback.
"""

import uuid
import falcon
import cla
from cla.utils import get_signing_service, get_signature_instance, get_project_instance, \
                      get_company_instance, get_user_instance
from cla.models import DoesNotExist

def request_individual_signature(project_id, user_id, return_url=None):
    """
    Handle POST request to send ICLA signature request to user.

    :param project_id: The project to sign for.
    :type project_id: string
    :param user_id: The ID of the user that will sign.
    :type user_id: string
    :param return_url: The URL to return the user to after signing is complete.
    :type return_url: string
    """
    return get_signing_service().request_individual_signature(str(project_id), str(user_id), return_url)

def request_corporate_signature(project_id, company_id, return_url=None):
    """
    Creates CCLA signature object that represents a company signing a CCLA.

    :param project_id: The ID of the project the company is signing a CCLA for.
    :type project_id: string
    :param company_id: The ID of the company that is signing the CCLA.
    :type company_id: string
    :param return_url: The URL to return the user to after signing is complete.
    :type return_url: string
    """
    return get_signing_service().request_corporate_signature(str(project_id), str(company_id), return_url)

def request_employee_signature(project_id, company_id, user_id, return_url=None):
    """
    Creates placeholder signature object that represents a user signing a CCLA as an employee.

    :param project_id: The ID of the project the user is signing a CCLA for.
    :type project_id: string
    :param company_id: The ID of the company the employee belongs to.
    :type company_id: string
    :param user_id: The ID of the user.
    :type user_id: string
    :param return_url: The URL to return the user to after signing is complete.
    """
    return get_signing_service().request_employee_signature(str(project_id), str(company_id), str(user_id), return_url)

def post_individual_signed(content, installation_id, github_repository_id, change_request_id):
    """
    Handle the posted callback from the signing service after ICLA signature.

    :param content: The POST body from the signing service callback.
    :type content: string
    :param repository_id: The ID of the repository that this signature was requested for.
    :type repository_id: string
    :param change_request_id: The ID of the change request or pull request that
        initiated this signature.
    :type change_request_id: string
    """
    get_signing_service().signed_individual_callback(content, installation_id, github_repository_id, change_request_id)

def post_corporate_signed(content, project_id, company_id):
    """
    Handle the posted callback from the signing service after CCLA signature.

    :param content: The POST body from the signing service callback.
    :type content: string
    :param project_id: The ID of the project that was signed.
    :type project_id: string
    :param company_id: The ID of the company that signed.
    :type company_id: string
    """
    get_signing_service().signed_corporate_callback(content, project_id, company_id)

def return_url(signature_id, event=None): # pylint: disable=unused-argument
    """
    Handle the GET request from the user once they have successfully signed.

    :param signature_id: The ID of the signature they have just signed.
    :type signature_id: string
    :param event: The event GET flag sent back from the signing service provider.
    :type event: string | None
    """
    try: # Load the signature based on ID.
        signature = get_signature_instance()
        signature.load(str(signature_id))
    except DoesNotExist as err:
        cla.log.error('Invalid signature_id provided when trying to send user back to their ' + \
                      'return_url after signing: %s', signature_id)
        return {'errors': {'signature_id': str(err)}}
    # Ensure everything went well on the signing service provider's side.
    if event is not None:
        # Expired signing URL - the user was redirected back immediately but still needs to sign.
        if event == 'ttl_expired' and not signature.get_signature_signed():
            # Need to re-generate a sign_url and try again.
            cla.log.info('DocuSign URL used was expired, re-generating sign_url')
            callback_url = signature.get_signature_callback_url()
            get_signing_service().populate_sign_url(signature, callback_url)
            signature.save()
            raise falcon.HTTPFound(signature.get_signature_sign_url())
    ret_url = signature.get_signature_return_url()
    if ret_url is not None:
        cla.log.info('Signature success - sending user to return_url: %s', ret_url)
        raise falcon.HTTPFound(ret_url)
    cla.log.info('No return_url set for signature - returning success message')
    return {'success': 'Thank you for signing'}
